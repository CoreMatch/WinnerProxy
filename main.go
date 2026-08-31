package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
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
	log.Printf("config loaded: %s", cfgPath)

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
	hrpauthCli := hrpauth.New(
		cfg.Upstreams.Hrpauth.URL,
		cfg.Upstreams.Hrpauth.ClientID,
		cfg.Upstreams.Hrpauth.ClientSecret,
		nil,
	)

	if cfg.Presence.Enabled {
		if cfg.Upstreams.Hrpauth.Enabled {
			announcePresence(hrpauthCli, cfg.Presence)
		} else {
			log.Printf("presence handshake skipped (hrpauth upstream disabled)")
		}
	}

	services := []proxy.UpstreamService{
		proxy.NewHrpauthService(hrpauthCli),
	}
	if cfg.Upstreams.Official.Enabled {
		mojang := proxy.NewMojangService(time.Duration(cfg.Upstreams.Official.TimeoutSec) * time.Second)
		services = append(services, mojang)
		log.Printf("mojang upstream enabled (timeout=%ds)", cfg.Upstreams.Official.TimeoutSec)
	}

	h := handler.New(services, hrpauthCli, buildCache(cfg.Cache))
	engine := router.New(h)

	log.Printf("WinnerProxy listening on %s", cfg.Server.Addr)
	if err := engine.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

// announcePresence performs the presence (bonjour) handshake with
// HRPAuth asynchronously. It registers WinnerProxy in HRPAuth's
// presence registry; a failure is only logged as a warning and never
// blocks or stops the main process.
func announcePresence(cli *hrpauth.Client, cfg config.PresenceConfig) {
	go func() {
		if err := cli.RegisterPresence(hrpauth.PresenceRequest{
			Name:       cfg.Name,
			TTLSeconds: cfg.TTLSeconds,
		}); err != nil {
			log.Printf("WARN: presence handshake with hrpauth failed (proxy continues running): %v", err)
			return
		}
		log.Printf("presence handshake ok: registered as %q", cfg.Name)
	}()
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
