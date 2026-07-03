package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds runtime configuration loaded from a YAML file.
type Config struct {
	// Addr is the HTTP server address, e.g. ":8080".
	Addr string `yaml:"addr"`
	// CacheSize is the freecache size in bytes.
	CacheSize int `yaml:"cache_size"`
}

// Default returns the built-in default configuration.
func Default() *Config {
	return &Config{
		Addr:      ":2779",
		CacheSize: 100 * 1024 * 1024, // 100 MB
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
