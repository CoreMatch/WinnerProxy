package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/winnerproxy/winnerproxy/config"
	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/handler"
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

	c := cache.New(cfg.CacheSize)
	log.Printf("freecache initialized: %d bytes", cfg.CacheSize)

	h := handler.New(c)
	engine := router.New(h)

	log.Printf("WinnerProxy listening on %s", cfg.Addr)
	if err := engine.Run(cfg.Addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

// preStart runs initialization steps that must complete before the
// server begins accepting connections. It ensures a config.yml exists
// next to the executable, creating a default one if missing.
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

// configPath returns the absolute path of config.yml located in the
// same directory as the running executable.
func configPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), configFileName), nil
}
