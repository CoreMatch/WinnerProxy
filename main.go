package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/winnerproxy/winnerproxy/config"
	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/handler"
	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
	"github.com/winnerproxy/winnerproxy/internal/router"
)

const configFileName = "config.yml"

func main() {
	noStdin := flag.Bool("no-stdin", false, "skip interactive manage-token prompt on first launch")
	flag.Parse()

	cfgPath, err := configPath()
	if err != nil {
		log.Fatalf("locate executable: %v", err)
	}

	preStart(cfgPath, *noStdin)

	cfg := config.Load(cfgPath)
	log.Printf("config loaded: %s", cfgPath)

	if cfg.Upstreams.Hrpauth.Enabled {
		log.Printf("hrpauth upstream enabled (url=%s, timeout=%ds)",
			cfg.Upstreams.Hrpauth.URL, cfg.Upstreams.Hrpauth.TimeoutSec)
		if cfg.Upstreams.Hrpauth.ManageToken == "" {
			log.Printf("WARN: hrpauth manage_token is empty; /register proxy-registration will fail. " +
				"Edit config.yml manually or restart with a TTY to enter it interactively.")
		}
	} else {
		log.Printf("hrpauth upstream disabled")
	}
	hrpauthCli := hrpauth.New(
		cfg.Upstreams.Hrpauth.URL,
		cfg.Upstreams.Hrpauth.ManageToken,
		nil,
	)

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

// preStart ensures config.yml exists. On a first run with a TTY stdin
// (and unless --no-stdin is set) it interactively asks for the
// HRPAuth Manage Token and writes it back to manage_token so
// subsequent runs pick it up automatically.
func preStart(cfgPath string, noStdin bool) {
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

	if noStdin {
		log.Printf("--no-stdin flag set, skipping manage-token prompt; edit %s manually", cfgPath)
		return
	}
	if !isTerminal(os.Stdin) {
		log.Printf("stdin is not a TTY, skipping manage-token prompt; edit %s manually", cfgPath)
		return
	}

	fmt.Fprintf(os.Stderr, "Enter HRPAuth Manage Token (or press Enter to skip): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return
	}
	token := strings.TrimSpace(scanner.Text())
	if token == "" {
		log.Printf("no manage token entered; edit %s manually before production use", cfgPath)
		return
	}

	if err := patchManageToken(cfgPath, token); err != nil {
		log.Printf("patch manage_token into config: %v", err)
		return
	}
	log.Printf("manage_token written to %s", cfgPath)
}

// patchManageToken loads cfg, sets manage_token, and writes back.
func patchManageToken(path, token string) error {
	cfg := config.Load(path)
	cfg.Upstreams.Hrpauth.ManageToken = token
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// isTerminal reports whether f is a character device (i.e. an
// interactive TTY) so we know whether to attempt the stdin prompt.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
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
