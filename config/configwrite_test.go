package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const rewriteFixture = `# WinnerProxy deployment config
version: "2"

server:
    addr: ":2777"

hrpauth:
    url: http://hrpauth:2778
    client_id: hrpauth-internal-super
    client_secret: super-secret
    presence:
        enabled: true
        name: WinnerProxy

upstreams:
    official:
        url: https://api.minecraftservices.com
        enabled: true
    hrpauth:
        url: http://hrpauth:2778/yggdrasil-api/
        client_id: hrpauth-internal-super
        client_secret: super-secret
        enabled: true
`

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestRewriteClientCredentialsReplacesMatchingPairs(t *testing.T) {
	path := writeFixture(t, rewriteFixture)

	n, err := RewriteClientCredentials(path, "hrpauth-internal-super", "super-secret", "WinnerProxy", "low-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("sections replaced = %d, want 2 (hrpauth + upstreams.hrpauth)", n)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "super-secret") {
		t.Error("rewritten config still contains the super secret")
	}
	if strings.Count(text, "low-secret") != 2 || strings.Count(text, "client_id: WinnerProxy") != 2 {
		t.Errorf("expected both sections to carry the new pair, got:\n%s", text)
	}
	if !strings.Contains(text, "# WinnerProxy deployment config") {
		t.Error("comments must survive the rewrite")
	}

	// Backup preserves the original content.
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(bak) != rewriteFixture {
		t.Error(".bak must contain the pre-change content")
	}

	// Permissions tightened on both files.
	for _, p := range []string{path, path + ".bak"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("%s mode = %v, want 0600", p, info.Mode().Perm())
		}
	}

	// The rewritten file must load through the regular Load path.
	cfg := Load(path)
	if cfg.Hrpauth.ClientID != "WinnerProxy" || cfg.Hrpauth.ClientSecret != "low-secret" {
		t.Errorf("hrpauth section = %q/%q, want the new pair", cfg.Hrpauth.ClientID, cfg.Hrpauth.ClientSecret)
	}
	if cfg.Upstreams.Hrpauth.ClientID != "WinnerProxy" || cfg.Upstreams.Hrpauth.ClientSecret != "low-secret" {
		t.Errorf("upstreams.hrpauth section = %q/%q, want the new pair", cfg.Upstreams.Hrpauth.ClientID, cfg.Upstreams.Hrpauth.ClientSecret)
	}
}

func TestRewriteClientCredentialsLeavesDifferentPairs(t *testing.T) {
	path := writeFixture(t, strings.Replace(rewriteFixture,
		"        client_id: hrpauth-internal-super\n        client_secret: super-secret\n        enabled: true",
		"        client_id: other-client\n        client_secret: other-secret\n        enabled: true", 1))

	n, err := RewriteClientCredentials(path, "hrpauth-internal-super", "super-secret", "WinnerProxy", "low-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("sections replaced = %d, want 1 (only the matching hrpauth section)", n)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "other-secret") {
		t.Error("a section with different credentials must stay untouched")
	}
}

func TestRewriteClientCredentialsNoMatchIsNoop(t *testing.T) {
	path := writeFixture(t, rewriteFixture)
	before, _ := os.ReadFile(path)

	n, err := RewriteClientCredentials(path, "nobody", "nothing", "WinnerProxy", "low-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("sections replaced = %d, want 0", n)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("no-match rewrite must not touch the file")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error("no-match rewrite must not create a backup")
	}
}

func TestRewriteClientCredentialsRejectsEmptyValues(t *testing.T) {
	path := writeFixture(t, rewriteFixture)
	if _, err := RewriteClientCredentials(path, "", "", "a", "b"); err == nil {
		t.Fatal("expected an error for empty old credentials")
	}
	if _, err := RewriteClientCredentials(path, "x", "y", "", ""); err == nil {
		t.Fatal("expected an error for empty new credentials")
	}
}

func TestRewriteClientCredentialsInvalidSource(t *testing.T) {
	path := writeFixture(t, "not: [valid yaml")
	if _, err := RewriteClientCredentials(path, "a", "b", "c", "d"); err == nil {
		t.Fatal("expected an error for an unparseable config file")
	}
}

func TestRewriteClientCredentialsWriteFailureKeepsFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits only")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(rewriteFixture), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	if _, err := RewriteClientCredentials(path, "hrpauth-internal-super", "super-secret", "WinnerProxy", "low-secret"); err == nil {
		t.Fatal("expected an error when the directory is read-only")
	}
	out, _ := os.ReadFile(path)
	if string(out) != rewriteFixture {
		t.Error("a failed write-back must leave the original file intact")
	}
}

func TestIsRenderedDeployment(t *testing.T) {
	if IsRenderedDeployment() {
		t.Fatal("no render envs set, must report false")
	}
	t.Setenv(RenderedConfigTemplateEnv, "/etc/ha/config.yaml.tmpl")
	t.Setenv(RenderedConfigPathEnv, "/app/config.yml")
	if !IsRenderedDeployment() {
		t.Fatal("both render envs set, must report true")
	}
}
