package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/winnerproxy/winnerproxy/config"
	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/handler"
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

	p := proxy.New(cfg)
	log.Printf("proxy initialized with %d upstream services", len(p.GetServices()))

	m := mapping.New(c, cfg, p)
	log.Printf("mapping initialized")

	h := handler.New(c, p, m)
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
