package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetryDefaults(t *testing.T) {
	r := Default().Hrpauth.Retry
	if r.StartupAttempts != 10 || r.StartupIntervalSec != 5 || r.ReconnectIntervalSec != 5 {
		t.Errorf("retry defaults = %+v, want 10/5/5", r)
	}
}

func TestRetryNormalized(t *testing.T) {
	got := RetryConfig{}.Normalized()
	if got.StartupAttempts != 10 || got.StartupIntervalSec != 5 || got.ReconnectIntervalSec != 5 {
		t.Errorf("Normalized(zero) = %+v, want 10/5/5", got)
	}
	custom := RetryConfig{StartupAttempts: 3, StartupIntervalSec: 2, ReconnectIntervalSec: 7}
	if got := custom.Normalized(); got != custom {
		t.Errorf("Normalized(custom) = %+v, want unchanged %+v", got, custom)
	}
}

func TestLoadRetryFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	yaml := "hrpauth:\n  retry:\n    startup_attempts: 3\n    startup_interval_sec: 2\n    reconnect_interval_sec: 7\n"
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	r := Load(path).Hrpauth.Retry
	if r.StartupAttempts != 3 || r.StartupIntervalSec != 2 || r.ReconnectIntervalSec != 7 {
		t.Errorf("loaded retry = %+v, want 3/2/7", r)
	}
}

func TestLoadRetryAbsentKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("server:\n  addr: \":2777\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := Load(path).Hrpauth.Retry
	if r.StartupAttempts != 10 || r.StartupIntervalSec != 5 || r.ReconnectIntervalSec != 5 {
		t.Errorf("retry with absent section = %+v, want defaults 10/5/5", r)
	}
}