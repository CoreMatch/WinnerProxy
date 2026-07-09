// Package config loads WinnerProxy's runtime configuration from a YAML
// file. Fields are grouped into nested sections so the on-disk
// structure mirrors the example layout in
// docs/DEVELOPMENT-ROADMAP §8.3.
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds runtime configuration loaded from a YAML file.
type Config struct {
	// Server controls the HTTP listener.
	Server ServerConfig `yaml:"server"`
	// Log controls log output behavior.
	Log LogConfig `yaml:"log"`
	// Upstreams groups all upstream Mojang / HRPAuth endpoints.
	Upstreams UpstreamsConfig `yaml:"upstreams"`
	// Site holds non-runtime metadata about the deployment.
	Site SiteConfig `yaml:"site"`
	// Version is the schema version of this config file.
	Version string `yaml:"version"`
}

// ServerConfig is the HTTP listener configuration.
type ServerConfig struct {
	// Addr is the address the engine binds to, e.g. ":2779".
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

// HrpauthConfig is the HRPAuth-specific upstream config. The
// ManageToken must match HRPAuth's config.yaml > manage.token.
type HrpauthConfig struct {
	// URL is the HRPAuth base URL.
	URL string `yaml:"url"`
	// ManageToken is the M.T. sent as the remember_token body field on
	// every M.T.-authenticated /register call.
	ManageToken string `yaml:"manage_token"`
	// TimeoutSec is the per-request timeout in seconds.
	TimeoutSec int `yaml:"timeout_sec"`
	// Enabled toggles whether this upstream is active.
	Enabled bool `yaml:"enabled"`
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
			Addr:            ":2779",
			ReadTimeoutSec:  15,
			WriteTimeoutSec: 15,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Upstreams: UpstreamsConfig{
			Official: UpstreamConfig{
				URL:        "https://api.minecraftservices.com",
				TimeoutSec: 10,
				Enabled:    true,
			},
			Hrpauth: HrpauthConfig{
				URL:         "http://127.0.0.1:2880",
				ManageToken: "",
				TimeoutSec:  10,
				Enabled:     true,
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
// fall back to Default() values; an unreadable or missing file yields
// the defaults without error.
func Load(path string) *Config {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
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
