package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSecretsRefusesUndecodableKeyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets", "pairing.key"), []byte("not base64!"), 0600); err != nil {
		t.Fatal(err)
	}
	c := Defaults()
	c.DataDir = dir
	err := loadSecrets(&c)
	if err == nil {
		t.Fatal("undecodable key file must be an error, not an empty secret")
	}
	if !strings.Contains(err.Error(), "pairing.key") {
		t.Fatalf("error must name the file: %v", err)
	}
}

func TestLoadSecretsCreatesAndRereads(t *testing.T) {
	dir := t.TempDir()
	c := Defaults()
	c.DataDir = dir
	if err := loadSecrets(&c); err != nil {
		t.Fatal(err)
	}
	first := c.Secrets.PairingSecret
	if len(first) != 32 || len(c.Secrets.ServerSaltKey) != 32 {
		t.Fatalf("want 32 raw bytes each, got %d and %d", len(first), len(c.Secrets.ServerSaltKey))
	}
	again := Defaults()
	again.DataDir = dir
	if err := loadSecrets(&again); err != nil || again.Secrets.PairingSecret != first {
		t.Fatalf("second load differs: err=%v", err)
	}
}

// Files written by the pre-library loader: base64 of 32 bytes, sometimes with
// a trailing newline from an operator's editor. They must keep loading.
func TestLoadSecretsReadsPreLibraryFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte(strings.Repeat("s", 32))
	if err := os.WriteFile(filepath.Join(dir, "secrets", "serversalt.key"), []byte(base64.StdEncoding.EncodeToString(raw)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := Defaults()
	c.DataDir = dir
	if err := loadSecrets(&c); err != nil {
		t.Fatal(err)
	}
	if c.Secrets.ServerSaltKey != string(raw) {
		t.Fatal("pre-library key file did not load to the same bytes")
	}
}

func TestLoadSecretsSkipsWhenBothConfigured(t *testing.T) {
	c := Defaults()
	c.DataDir = "/nonexistent/never-created"
	c.Secrets.PairingSecret = strings.Repeat("p", 32)
	c.Secrets.ServerSaltKey = strings.Repeat("s", 32)
	if err := loadSecrets(&c); err != nil {
		t.Fatalf("configured secrets must not touch the data dir: %v", err)
	}
}
