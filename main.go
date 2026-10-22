package main

import (
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
			log.Printf("WARN: hrpauth client_id or client_secret is empty; proxy-authenticated requests will fail. " +
				"Edit config.yml manually.")
		}
	} else {
		log.Printf("hrpauth upstream disabled")
	}
	hrpauthProxyCli := hrpauth.New(
		cfg.Upstreams.Hrpauth.URL,
		cfg.Upstreams.Hrpauth.ClientID,
		cfg.Upstreams.Hrpauth.ClientSecret,
		nil,
	)

	// Management client for Presence, SDK, and Relay registration.
	// Uses cfg.Hrpauth which points to the root backend.
	hrpauthMgmtCli := hrpauth.New(
		cfg.Hrpauth.URL,
		cfg.Hrpauth.ClientID,
		cfg.Hrpauth.ClientSecret,
		nil,
	)

	// 2.2 Upload the compile-time SDK package to HRPAuth (if configured),
	// non-blocking. The WEBUI SDK handler aggregates it into the frontend
	// build.
	if cfg.Hrpauth.SDK.Enabled {
		uploadSDKPackage(hrpauthMgmtCli)
	}

	// 2.1 Presence handshake with HRPAuth (bonjour), non-blocking.
	// Registers the service and its relay rules.
	if cfg.Hrpauth.Presence.Enabled {
		// Fetch server meta from upstream HRPAuth to include in RootMetadata.
		// This allows HRPAuth to aggregate WinnerProxy's Yggdrasil info.
		meta, _ := hrpauthProxyCli.GetServerMeta()
		announcePresence(hrpauthMgmtCli, cfg.Hrpauth.Presence, cfg.Server.PublicURL, meta)
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

// announcePresence performs the presence (bonjour) handshake with
// HRPAuth asynchronously. It registers WinnerProxy in HRPAuth's
// presence registry and registers relay rules; a failure is only logged
// as a warning and never blocks or stops the main process.
func announcePresence(cli *hrpauth.Client, cfg config.PresenceConfig, publicURL string, meta map[string]any) {
	go func() {
		req := hrpauth.PresenceRequest{
			Name:          cfg.Name,
			TTLSeconds:    cfg.TTLSeconds,
			SecurityLevel: 0, // Level 0 to allow anonymous relay for Yggdrasil
			Scope: &hrpauth.PresenceScope{
				Name:          "winnerproxy",
				FrontendAreas: []string{"webui-dash"},
			},
			RootMetadata: map[string]interface{}{
				"yggdrasil": meta,
			},
		}
		if err := cli.RegisterPresence(req); err != nil {
			log.Printf("WARN: presence handshake with hrpauth failed (proxy continues running): %v", err)
			return
		}
		log.Printf("presence handshake ok: registered as %q", cfg.Name)

		if publicURL == "" {
			return
		}

		relays := buildRelayRules(publicURL, "/yggdrasil")
		if err := cli.RegisterRelay(cfg.Name, relays); err != nil {
			log.Printf("WARN: relay registration failed (proxy continues running): %v", err)
			return
		}
		log.Printf("relay registration ok: %d paths registered under /yggdrasil", len(relays))
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
		"/register",
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
