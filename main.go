package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/winnerproxy/winnerproxy/config"
	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/handler"
	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
	"github.com/winnerproxy/winnerproxy/internal/mapping"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
	"github.com/winnerproxy/winnerproxy/internal/router"
)

const configFileName = "config.yml"

func main() {
	cfgPath, err := configPath()
	if err != nil {
		log.Fatalf("locate executable: %v", err)
	}

	preStart(cfgPath)

	cfg := config.Load(cfgPath)
	log.Printf("config loaded: %s", cfgPath)

	c := cache.New(cfg.Cache.Size)
	log.Printf("freecache initialized: %d bytes", cfg.Cache.Size)

	// TODO(P6): wire from cfg.Upstreams.Hrpauth
	hrpauthCli := hrpauth.New("", "", nil)
	services := []proxy.UpstreamService{
		proxy.NewHrpauthService(hrpauthCli),
	}

	if cfg.Upstreams.Official.Enabled {
		mojang := proxy.NewMojangService(time.Duration(cfg.Upstreams.Official.TimeoutSec) * time.Second)
		services = append(services, mojang)
		log.Printf("mojang upstream enabled (timeout=%ds)", cfg.Upstreams.Official.TimeoutSec)
	}

	m := mapping.New(c, cfg, services)
	log.Printf("mapping initialized with %d services", len(services))

	h := handler.New(services, hrpauthCli, m)
	engine := router.New(h)

	log.Printf("WinnerProxy listening on %s", cfg.Server.Addr)
	if err := engine.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func preStart(cfgPath string) {
	log.Printf("running pre-start process: %s", cfgPath)
	if _, err := os.Stat(cfgPath); err == nil {
		log.Printf("config already present: %s", cfgPath)
		return
	} else if !os.IsNotExist(err) {
		log.Printf("stat config: %v", err)
		return
	}
	if err := os.WriteFile(cfgPath, config.DefaultYAML(), 0o644); err != nil {
		log.Printf("create config: %v", err)
		return
	}
	log.Printf("created default config: %s", cfgPath)
}

func configPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), configFileName), nil
}
