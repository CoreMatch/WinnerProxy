package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/winnerproxy/winnerproxy/config"
	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/handler"
	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
	"github.com/winnerproxy/winnerproxy/internal/router"
)

const configFileName = "config.yml"

func main() {
	flag.Parse()

	cfgPath, err := configPath()
	if err != nil {
		log.Fatalf("locate executable: %v", err)
	}

	cfg := config.Load(cfgPath)
	if _, ferr := os.Stat(cfgPath); ferr == nil {
		log.Printf("config loaded: %s", cfgPath)
	}

	if cfg.Upstreams.Hrpauth.Enabled {
		log.Printf("hrpauth upstream enabled (url=%s, timeout=%ds)",
			cfg.Upstreams.Hrpauth.URL, cfg.Upstreams.Hrpauth.TimeoutSec)
		if cfg.Upstreams.Hrpauth.ClientID == "" || cfg.Upstreams.Hrpauth.ClientSecret == "" {
			log.Fatalf("hrpauth upstream is enabled but upstreams.hrpauth.client_id/client_secret is empty: " +
				"WinnerProxy cannot obtain a service token (needed for the X-Internal-Key used by proxy " +
				"registration). Fill both values in config.yml, or set upstreams.hrpauth.enabled: false.")
		}
	} else {
		log.Printf("hrpauth upstream disabled")
	}

	// Management client for Presence, SDK, and Relay registration.
	// Uses cfg.Hrpauth which points to the root backend.
	// Fallback to upstream credentials if management credentials are not set.
	mgmtURL := cfg.Hrpauth.URL
	mgmtClientID := cfg.Hrpauth.ClientID
	mgmtClientSecret := cfg.Hrpauth.ClientSecret

	// If ClientID is missing, we assume the user hasn't configured the new 'hrpauth' section
	// and we fallback to 'upstreams.hrpauth' credentials and URL.
	if mgmtClientID == "" {
		log.Printf("Presence: management credentials not set, falling back to upstream credentials")
		mgmtClientID = cfg.Upstreams.Hrpauth.ClientID
		mgmtClientSecret = cfg.Upstreams.Hrpauth.ClientSecret
		// Also fallback URL if the management URL is just the default
		if mgmtURL == "http://127.0.0.1:2778" && cfg.Upstreams.Hrpauth.URL != "" {
			// Strip /yggdrasil-api/ if present in upstream URL to get the root
			lowerURL := strings.ToLower(cfg.Upstreams.Hrpauth.URL)
			if idx := strings.Index(lowerURL, "/yggdrasil-api"); idx != -1 {
				mgmtURL = cfg.Upstreams.Hrpauth.URL[:idx]
			} else {
				mgmtURL = cfg.Upstreams.Hrpauth.URL
			}
			log.Printf("Presence: falling back to upstream URL (root): %s", mgmtURL)
		}
	} else {
		log.Printf("Presence: using dedicated management credentials (client_id=%s)", mgmtClientID)
	}

	if mgmtClientID == "" {
		log.Printf("WARN: No HRPAuth credentials found in config. Management tasks will fail.")
	}

	// Startup credential resolution: when the configured credentials are
	// HRPAuth's built-in super client, they are exchanged for
	// WinnerProxy's own low-privilege client via POST /oauth/register
	// (RFC 7591 protected registration) and the super secret is discarded
	// from memory. Credentials that are already a per-service client are
	// reused unchanged (idempotent path — no exchange, no write-back).
	// Connection-level failures are retried (hrpauth.retry.startup_attempts,
	// default 10, every startup_interval_sec). If HRPAuth is still
	// unreachable afterwards the proxy starts in degraded mode and a
	// background loop reconnects every reconnect_interval_sec; the
	// unverified (possibly super) credentials are kept out of every client
	// until resolution succeeds. Authoritative credential rejections still
	// abort startup: super credentials must never reach business requests.
	retryCfg := cfg.Hrpauth.Retry.Normalized()
	resolved := true
	var superID, superSecret string
	if mgmtClientID != "" && mgmtClientSecret != "" {
		superID, superSecret = mgmtClientID, mgmtClientSecret
		res, err := resolveCredentialsWithRetry(mgmtURL, cfg, mgmtClientID, mgmtClientSecret, retryCfg)
		if err != nil {
			resolved = false
			log.Printf("WARN: HRPAuth unreachable after %d startup attempts: %v — starting in degraded mode (HRPAuth integration disabled), reconnecting every %ds",
				retryCfg.StartupAttempts, err, retryCfg.ReconnectIntervalSec)
		} else if res.Exchanged {
			log.Printf("credentials: super client detected and exchanged for per-service client %q; super credentials discarded from memory", res.ClientID)
			applyResolvedCredentials(cfg, cfgPath, superID, superSecret, res)
			mgmtClientID, mgmtClientSecret = res.ClientID, res.ClientSecret
		} else {
			log.Printf("credentials: configured client %q confirmed as per-service client (no exchange needed)", mgmtClientID)
		}
	}

	proxyID, proxySecret := cfg.Upstreams.Hrpauth.ClientID, cfg.Upstreams.Hrpauth.ClientSecret
	mgmtID, mgmtSecret := mgmtClientID, mgmtClientSecret
	if !resolved {
		// Unverified credentials must not reach business or management
		// requests; the reconnect loop installs the verified pair via
		// SetCredentials once HRPAuth answers.
		proxyID, proxySecret = "", ""
		mgmtID, mgmtSecret = "", ""
	}

	hrpauthProxyCli := hrpauth.New(
		cfg.Upstreams.Hrpauth.URL,
		proxyID,
		proxySecret,
		nil,
	)
	// The proxy-registration key is issued per presence-registry name.
	hrpauthProxyCli.SetServiceName(cfg.Hrpauth.Presence.Name)

	hrpauthMgmtCli := hrpauth.New(
		mgmtURL,
		mgmtID,
		mgmtSecret,
		nil,
	)

	if resolved {
		// 2.2 Upload the compile-time SDK package to HRPAuth (if configured),
		// non-blocking. The WEBUI SDK handler aggregates it into the frontend
		// build.
		if cfg.Hrpauth.SDK.Enabled {
			uploadSDKPackage(hrpauthMgmtCli)
		}

		// 2.1 Presence handshake with HRPAuth (bonjour), non-blocking.
		// Registers the service and its relay rules, then fetches the
		// X-Internal-Key used by proxy registration.
		if cfg.Hrpauth.Presence.Enabled {
			announcePresence(hrpauthMgmtCli, hrpauthProxyCli, cfg.Hrpauth.Presence, cfg.Server.PublicURL)
		}
	} else {
		go reconnectToHrpauth(mgmtURL, cfg, cfgPath, superID, superSecret,
			hrpauthMgmtCli, hrpauthProxyCli, time.Duration(retryCfg.ReconnectIntervalSec)*time.Second)
	}

	services := []proxy.UpstreamService{
		proxy.NewHrpauthService(hrpauthProxyCli),
	}
	if cfg.Upstreams.Official.Enabled {
		mojang := proxy.NewMojangService(
			cfg.Upstreams.Official.URL,
			time.Duration(cfg.Upstreams.Official.TimeoutSec)*time.Second,
		)
		services = append(services, mojang)
		log.Printf("mojang upstream enabled (url=%s, timeout=%ds)",
			cfg.Upstreams.Official.URL, cfg.Upstreams.Official.TimeoutSec)
	}

	h := handler.New(services, hrpauthProxyCli, buildCache(cfg.Cache))
	engine := router.New(h)

	log.Printf("WinnerProxy listening on %s", cfg.Server.Addr)
	if err := engine.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

// resolveCredentialsWithRetry runs the presence bootstrap plus credential
// resolution, retrying connection-level failures (hrpauth.ErrUpstream) up
// to retry.StartupAttempts times, retry.StartupIntervalSec apart. An
// authoritative rejection aborts startup immediately — retrying cannot
// fix wrong credentials. When all attempts fail with connection errors
// the last error is returned so the caller can start in degraded mode.
func resolveCredentialsWithRetry(mgmtURL string, cfg *config.Config, clientID, clientSecret string, retry config.RetryConfig) (*hrpauth.Resolution, error) {
	interval := time.Duration(retry.StartupIntervalSec) * time.Second
	var lastErr error
	for attempt := 1; attempt <= retry.StartupAttempts; attempt++ {
		// The register endpoint only accepts presence-registered service
		// names, so the bonjour handshake goes first. It is a plain
		// request: no token may be attached before the credentials are
		// resolved (the super secret is used for the exchange call only).
		if cfg.Hrpauth.Presence.Enabled {
			if err := hrpauth.RegisterPresencePlain(mgmtURL, buildPresenceRequest(cfg.Hrpauth.Presence, nil), nil); err != nil {
				log.Printf("Presence: bootstrap registration failed: %v (a super-credential exchange will be rejected until presence succeeds)", err)
			}
		}

		res, err := hrpauth.ResolveServiceCredentials(mgmtURL, cfg.Hrpauth.Presence.Name, clientID, clientSecret, nil)
		if err == nil {
			return res, nil
		}
		if !errors.Is(err, hrpauth.ErrUpstream) {
			log.Fatalf("credential resolution failed (refusing to start with unverified or super credentials): %v", err)
		}
		lastErr = err
		if attempt < retry.StartupAttempts {
			log.Printf("HRPAuth unreachable (attempt %d/%d): %v — retrying in %ds",
				attempt, retry.StartupAttempts, err, retry.StartupIntervalSec)
			time.Sleep(interval)
		}
	}
	return nil, lastErr
}

// reconnectToHrpauth is the degraded-startup recovery loop: it retries
// the presence bootstrap plus credential resolution every interval until
// HRPAuth answers. On success the verified credentials are installed into
// both clients and the deferred SDK/presence registrations run. An
// authoritative credential rejection stops the loop with a loud error —
// the proxy keeps serving, but the HRPAuth integration stays disabled
// until the config is fixed and the process restarted.
func reconnectToHrpauth(mgmtURL string, cfg *config.Config, cfgPath, superID, superSecret string, mgmtCli, proxyCli *hrpauth.Client, interval time.Duration) {
	for {
		time.Sleep(interval)
		if cfg.Hrpauth.Presence.Enabled {
			if err := hrpauth.RegisterPresencePlain(mgmtURL, buildPresenceRequest(cfg.Hrpauth.Presence, nil), nil); err != nil {
				log.Printf("HRPAuth reconnect: presence bootstrap failed: %v — next attempt in %s", err, interval)
				continue
			}
		}
		res, err := hrpauth.ResolveServiceCredentials(mgmtURL, cfg.Hrpauth.Presence.Name, superID, superSecret, nil)
		if err != nil {
			if errors.Is(err, hrpauth.ErrUpstream) {
				log.Printf("HRPAuth reconnect failed: %v — next attempt in %s", err, interval)
				continue
			}
			log.Printf("ERROR: HRPAuth reconnect gave up: %v — HRPAuth integration stays disabled; fix the credentials/config and restart", err)
			return
		}
		if res.Exchanged {
			log.Printf("credentials: super client detected and exchanged for per-service client %q; super credentials discarded from memory", res.ClientID)
			applyResolvedCredentials(cfg, cfgPath, superID, superSecret, res)
		} else {
			log.Printf("credentials: configured client %q confirmed as per-service client (no exchange needed)", superID)
		}
		mgmtCli.SetCredentials(res.ClientID, res.ClientSecret)
		proxyCli.SetCredentials(cfg.Upstreams.Hrpauth.ClientID, cfg.Upstreams.Hrpauth.ClientSecret)
		log.Printf("HRPAuth reconnect: credentials resolved, resuming SDK/presence registration")
		if cfg.Hrpauth.SDK.Enabled {
			uploadSDKPackage(mgmtCli)
		}
		if cfg.Hrpauth.Presence.Enabled {
			announcePresence(mgmtCli, proxyCli, cfg.Hrpauth.Presence, cfg.Server.PublicURL)
		}
		return
	}
}

// announcePresence performs the presence (bonjour) handshake with
// HRPAuth asynchronously. It registers WinnerProxy in HRPAuth's
// presence registry, registers relay rules, and fetches the
// X-Internal-Key used by proxy registration; a failure is only logged
// and never blocks or stops the main process.
func announcePresence(cli, proxyCli *hrpauth.Client, cfg config.PresenceConfig, publicURL string) {
	go func() {
		log.Printf("Presence: starting microservice registration (name=%s)", cfg.Name)

		// Fetch server meta from upstream HRPAuth to include in RootMetadata.
		// This allows HRPAuth to aggregate WinnerProxy's Yggdrasil info.
		// We do this inside the goroutine to avoid blocking startup. The
		// call re-upserts the presence record that the synchronous
		// bootstrap handshake already created (with root_metadata this
		// time).
		meta, _ := proxyCli.GetServerMeta()

		if err := cli.RegisterPresence(buildPresenceRequest(cfg, meta)); err != nil {
			log.Printf("Presence: registration failed: %v", err)
			return
		}
		log.Printf("Presence: microservice %q successfully registered (TTL=%ds)", cfg.Name, cfg.TTLSeconds)

		// The internal key is issued per registered service; it authenticates
		// POST /internal/proxy-register. Without it proxy registration cannot
		// succeed, so surface the failure explicitly.
		if err := proxyCli.FetchInternalKey(); err != nil {
			log.Printf("ERROR: internal key fetch failed: %v — proxy registration will fail until a key is fetched", err)
		} else {
			log.Printf("Presence: internal key fetched for %q", cfg.Name)
		}

		if publicURL == "" {
			log.Printf("Presence: skipping relay registration (public_url not configured)")
			return
		}

		log.Printf("Presence: initiating relay registration for %s", publicURL)
		relays := buildRelayRules(publicURL, "/winnerproxy")
		if err := cli.RegisterRelay(cfg.Name, relays); err != nil {
			log.Printf("Presence: relay registration failed: %v", err)
			return
		}
		log.Printf("Presence: %d relay rules successfully active under /winnerproxy", len(relays))
	}()
}

// buildRelayRules creates a set of relay mappings for Yggdrasil endpoints.
func buildRelayRules(base, prefix string) []hrpauth.RelayRule {
	base = strings.TrimRight(base, "/")
	prefix = strings.TrimRight(prefix, "/")
	paths := []string{
		"/",
		"/authserver",
		"/sessionserver",
		"/api/profiles/minecraft",
		"/textures",
		"/previews",
		"/texture",
		"/skin",
		"/skins",
		"/minecraftservices",
	}

	relays := make([]hrpauth.RelayRule, 0, len(paths))
	for _, path := range paths {
		relays = append(relays, hrpauth.RelayRule{
			Dest:   prefix + path,
			Source: base + path,
		})
	}
	return relays
}

// buildCache returns a ProfileCache from config. size=0 yields the
// noop cache; otherwise a freecache-backed instance is constructed.
func buildCache(cfg config.CacheConfig) cache.ProfileCache {
	if cfg.Size <= 0 {
		log.Printf("profile cache disabled (cache.size=0)")
		return cache.NewNoop()
	}
	ttl := time.Duration(cfg.TTLSec) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return cache.NewFreeCache(cfg.Size, ttl)
}

func configPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), configFileName), nil
}

// buildPresenceRequest assembles the presence (bonjour) body. meta may be
// nil during the synchronous bootstrap handshake (root_metadata is added
// by the asynchronous follow-up once the upstream meta is known).
func buildPresenceRequest(cfg config.PresenceConfig, meta map[string]interface{}) hrpauth.PresenceRequest {
	req := hrpauth.PresenceRequest{
		Name:          cfg.Name,
		TTLSeconds:    cfg.TTLSeconds,
		SecurityLevel: 0, // Level 0 to allow anonymous relay for Yggdrasil
		Scope: &hrpauth.PresenceScope{
			Name:          "winnerproxy",
			FrontendAreas: []string{"webui-dash"},
		},
	}
	if meta != nil {
		req.RootMetadata = map[string]interface{}{
			"yggdrasil": meta,
		}
	}
	return req
}

// applyResolvedCredentials completes a successful super-credential
// exchange: every in-memory config field that carried the super pair is
// overwritten with the exchanged low-privilege pair (so no client is ever
// constructed with the super secret and no live reference to it remains),
// then the config-file write-back is attempted.
//
// Write-back outcomes:
//   - entrypoint-rendered deployment (CONFIG_TEMPLATE/RENDERED_CONFIG):
//     the file would be re-rendered on the next container start, so
//     nothing is written and the operator is pointed at the
//     template/environment side;
//   - write failure: not fatal — the service keeps running with the
//     in-memory credentials, but the next startup will exchange again;
//   - success: the file holds the low-privilege pair (atomic rename,
//     .bak of the previous content, permissions tightened to 0600), so
//     the next startup takes the idempotent path directly.
func applyResolvedCredentials(cfg *config.Config, cfgPath, superID, superSecret string, res *hrpauth.Resolution) {
	if cfg.Hrpauth.ClientID == superID && cfg.Hrpauth.ClientSecret == superSecret {
		cfg.Hrpauth.ClientID = res.ClientID
		cfg.Hrpauth.ClientSecret = res.ClientSecret
	}
	if cfg.Upstreams.Hrpauth.ClientID == superID && cfg.Upstreams.Hrpauth.ClientSecret == superSecret {
		cfg.Upstreams.Hrpauth.ClientID = res.ClientID
		cfg.Upstreams.Hrpauth.ClientSecret = res.ClientSecret
	}

	if config.IsRenderedDeployment() {
		log.Printf("WARN: config file is entrypoint-rendered (CONFIG_TEMPLATE/RENDERED_CONFIG); per-service credentials were NOT written back — set WINNERPROXY_CLIENT_ID/WINNERPROXY_CLIENT_SECRET on the template/environment side instead; this deployment will exchange again on every start")
		return
	}
	n, err := config.RewriteClientCredentials(cfgPath, superID, superSecret, res.ClientID, res.ClientSecret)
	if err != nil {
		log.Printf("ERROR: config file could not be rewritten with the per-service credentials (the service keeps running with the in-memory credentials): %v — next startup will perform the exchange again", err)
		return
	}
	log.Printf("credentials: config file rewritten with per-service client %q (sections replaced=%d, previous content kept at %s.bak)", res.ClientID, n, cfgPath)
}
