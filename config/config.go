package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds runtime configuration loaded from a YAML file.
// Fields are grouped into nested sections so the on-disk structure
// mirrors the example layout (server / cache / log / proxy ...).
type Config struct {
	// Server controls the HTTP listener.
	Server ServerConfig `yaml:"server"`
	// Cache controls the in-memory freecache instance.
	Cache CacheConfig `yaml:"cache"`
	// Log controls log output behavior.
	Log LogConfig `yaml:"log"`
	// Proxy controls the upstream / callback behavior.
	Proxy ProxyConfig `yaml:"proxy"`
	// Upstreams groups all upstream Mojang / Yggdrasil endpoints.
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
	// the entire request, including the body. Zero means no timeout.
	ReadTimeoutSec int `yaml:"read_timeout_sec"`
	// WriteTimeoutSec is the maximum duration in seconds for writing
	// the response. Zero means no timeout.
	WriteTimeoutSec int `yaml:"write_timeout_sec"`
}

// CacheConfig configures the freecache-backed store.
type CacheConfig struct {
	// Size is the freecache size in bytes.
	Size int `yaml:"size"`
	// GCIntervalSec controls how often expired entries are swept.
	GCIntervalSec int `yaml:"gc_interval_sec"`
}

// LogConfig configures the process logger.
type LogConfig struct {
	// Level is the log level: debug, info, warn, error.
	Level string `yaml:"level"`
	// Format is "text" or "json".
	Format string `yaml:"format"`
}

// ProxyConfig configures the upstream / callback behavior.
type ProxyConfig struct {
	// CallbackURL is the upstream endpoint the proxy talks to.
	CallbackURL string `yaml:"callback_url"`
	// TimeoutSec is the upstream request timeout in seconds.
	TimeoutSec int `yaml:"timeout_sec"`
}

// SiteConfig is non-runtime metadata about this deployment.
type SiteConfig struct {
	// Name is the human-readable service name.
	Name string `yaml:"name"`
	// Version is the deployed build version.
	Version string `yaml:"version"`
}

// UpstreamConfig describes a single upstream endpoint the proxy can
// route requests to.
type UpstreamConfig struct {
	// URL is the upstream base URL.
	URL string `yaml:"url"`
	// TimeoutSec is the per-request timeout in seconds.
	TimeoutSec int `yaml:"timeout_sec"`
	// Enabled toggles whether this upstream is active.
	Enabled bool `yaml:"enabled"`
}

// UpstreamsConfig groups every upstream the proxy knows about.
type UpstreamsConfig struct {
	// Official is the upstream for the official Mojang services.
	Official UpstreamConfig `yaml:"official"`
	// YggdrasilAPI is the upstream for the Yggdrasil authentication API.
	YggdrasilAPI UpstreamConfig `yaml:"yggdrasilapi"`
}

// Default returns the built-in default configuration.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:            ":2779",
			ReadTimeoutSec:  15,
			WriteTimeoutSec: 15,
		},
		Cache: CacheConfig{
			Size:          100 * 1024 * 1024, // 100 MB
			GCIntervalSec: 60,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Proxy: ProxyConfig{
			CallbackURL: "",
			TimeoutSec:  10,
		},
		Site: SiteConfig{
			Name:    "WinnerProxy",
			Version: "0.1.0",
		},
		Upstreams: UpstreamsConfig{
			Official: UpstreamConfig{
				URL:        "https://api.minecraftservices.com",
				TimeoutSec: 10,
				Enabled:    true,
			},
			YggdrasilAPI: UpstreamConfig{
				URL:        "https://authserver.mojang.com",
				TimeoutSec: 10,
				Enabled:    true,
			},
		},
		Version: "1",
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
