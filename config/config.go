// Package config loads WinnerProxy's runtime configuration from a YAML
// file. Fields are grouped into nested sections so the on-disk
// structure mirrors the example layout in
// docs/DEVELOPMENT-ROADMAP §8.3.
package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds runtime configuration loaded from a YAML file.
type Config struct {
	// Server controls the HTTP listener.
	Server ServerConfig `yaml:"server"`
	// Cache controls the in-process profile cache.
	Cache CacheConfig `yaml:"cache"`
	// Log controls log output behavior.
	Log LogConfig `yaml:"log"`
	// Presence controls the microservice presence handshake with HRPAuth.
	Presence PresenceConfig `yaml:"presence"`
	// Upstreams groups all upstream Mojang / HRPAuth endpoints.
	Upstreams UpstreamsConfig `yaml:"upstreams"`
	// Site holds non-runtime metadata about the deployment.
	Site SiteConfig `yaml:"site"`
	// Version is the schema version of this config file.
	Version string `yaml:"version"`
}

// ServerConfig is the HTTP listener configuration.
type ServerConfig struct {
	// Addr is the address the engine binds to, e.g. ":2777".
	Addr string `yaml:"addr"`
	// ReadTimeoutSec is the maximum duration in seconds for reading
	// the entire request, including the body.
	ReadTimeoutSec int `yaml:"read_timeout_sec"`
	// WriteTimeoutSec is the maximum duration in seconds for writing
	// the response.
	WriteTimeoutSec int `yaml:"write_timeout_sec"`
}

// LogConfig configures the process logger.
type LogConfig struct {
	// Level is the log level: debug, info, warn, error.
	Level string `yaml:"level"`
	// Format is "text" or "json".
	Format string `yaml:"format"`
}

// CacheConfig configures the in-process profile cache (freecache).
// Set Size=0 to disable caching entirely.
type CacheConfig struct {
	// Size is the maximum cache size in bytes. Default 100 MiB.
	Size int `yaml:"size"`
	// TTLSec is how long a profile stays in the cache. Default 300s.
	TTLSec int `yaml:"ttl_sec"`
}

// SiteConfig is non-runtime metadata about this deployment.
type SiteConfig struct {
	// Name is the human-readable service name.
	Name string `yaml:"name"`
	// Version is the deployed build version.
	Version string `yaml:"version"`
}

// UpstreamConfig describes a generic upstream endpoint.
type UpstreamConfig struct {
	// URL is the upstream base URL.
	URL string `yaml:"url"`
	// TimeoutSec is the per-request timeout in seconds.
	TimeoutSec int `yaml:"timeout_sec"`
	// Enabled toggles whether this upstream is active.
	Enabled bool `yaml:"enabled"`
}

// HrpauthConfig is the HRPAuth-specific upstream config.
type HrpauthConfig struct {
	// URL is the HRPAuth base URL.
	URL string `yaml:"url"`
	// ClientID is the OAuth2 client ID for WinnerProxy.
	ClientID string `yaml:"client_id"`
	// ClientSecret is the OAuth2 client secret for WinnerProxy.
	ClientSecret string `yaml:"client_secret"`
	// TimeoutSec is the per-request timeout in seconds.
	TimeoutSec int `yaml:"timeout_sec"`
	// Enabled toggles whether this upstream is active.
	Enabled bool `yaml:"enabled"`
}

// PresenceConfig controls the microservice presence handshake with
// HRPAuth (POST /services/presence, the "bonjour" handshake). The
// handshake registers WinnerProxy in HRPAuth's in-process presence
// registry so the main service knows it is online. A failed handshake
// is logged but never blocks or stops the proxy.
type PresenceConfig struct {
	// Enabled toggles the presence handshake. Default true.
	Enabled bool `yaml:"enabled"`
	// Name is the service name registered in HRPAuth. Default "WinnerProxy".
	Name string `yaml:"name"`
	// TTLSeconds is the self-declared lifetime in seconds; <=0 (default)
	// means the record never expires and stays until HRPAuth stops.
	TTLSeconds int `yaml:"ttl_seconds"`
}

// UpstreamsConfig groups every upstream the proxy knows about.
type UpstreamsConfig struct {
	// Official is the upstream for the official Mojang services.
	Official UpstreamConfig `yaml:"official"`
	// Hrpauth is the upstream for the HRPAuth (HA) backend.
	Hrpauth HrpauthConfig `yaml:"hrpauth"`
}

// Default returns the built-in default configuration.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:            ":2777",
			ReadTimeoutSec:  15,
			WriteTimeoutSec: 15,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Presence: PresenceConfig{
			Enabled:    true,
			Name:       "WinnerProxy",
			TTLSeconds: 0,
		},
		Cache: CacheConfig{
			Size:   100 * 1024 * 1024, // 100 MiB
			TTLSec: 300,               // 5 minutes
		},
		Upstreams: UpstreamsConfig{
			Official: UpstreamConfig{
				URL:        "https://api.minecraftservices.com",
				TimeoutSec: 10,
				Enabled:    true,
			},
			Hrpauth: HrpauthConfig{
				URL:          "http://127.0.0.1:2778",
				ClientID:     "",
				ClientSecret: "",
				TimeoutSec:   10,
				Enabled:      true,
			},
		},
		Site: SiteConfig{
			Name:    "WinnerProxy",
			Version: "0.2.0",
		},
		Version: "2",
	}
}

// Load reads configuration from the YAML file at path. Missing fields
// fall back to Default() values. When the file does not exist a fresh
// default config.yml is written so the operator can edit it.
func Load(path string) *Config {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("config file not found at %s, creating default config...", path)
		if werr := os.WriteFile(path, DefaultYAML(), 0644); werr != nil {
			log.Printf("failed to write default config: %v, using built-in defaults", werr)
		} else {
			log.Printf("default config created at %s — please edit and restart", path)
		}
		return cfg
	}
	_ = yaml.Unmarshal(data, cfg)
	return cfg
}

// DefaultYAML returns the default configuration serialized as YAML,
// suitable for writing to a freshly created config file.
func DefaultYAML() []byte {
	out, _ := yaml.Marshal(Default())
	return out
}
