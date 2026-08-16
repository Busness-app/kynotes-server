package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yoshiofthewire/kynotes-server/internal/auth"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/storage"
)

func TestEnsureBootstrapAdminGenerated(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "kynotes.sqlite"))
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	cfg := config.Defaults()
	cfg.DataDir = dir
	cfg.Secrets.ServerSaltKey = "test-salt-key-32-bytes-long-1234"

	// 1. Initial bootstrap
	if err := EnsureBootstrapAdmin(store.DB(), cfg); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	passFile := filepath.Join(dir, "first-run-password.txt")
	content, err := os.ReadFile(passFile)
	if err != nil {
		t.Fatalf("failed to read first-run-password.txt: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "User: admin") || !strings.HasPrefix(lines[1], "Password: ") {
		t.Fatalf("unexpected first-run-password.txt content: %s", string(content))
	}
	password := strings.TrimPrefix(lines[1], "Password: ")

	// Verify database record
	var username, storedHash, salt string
	var iterations int
	var role, status string
	err = store.DB().QueryRow(`SELECT username, auth_secret_hash, login_salt, login_iterations, role, status FROM users WHERE username='admin'`).Scan(
		&username, &storedHash, &salt, &iterations, &role, &status,
	)
	if err != nil {
		t.Fatalf("admin user not found in DB: %v", err)
	}

	if role != "admin" || status != "active" {
		t.Fatalf("unexpected admin role/status: role=%s, status=%s", role, status)
	}

	// Verify credentials derive and verify correctly
	authSecret, err := auth.DeriveAuthSecret(password, salt, iterations)
	if err != nil {
		t.Fatalf("failed to derive auth secret: %v", err)
	}
	if err := auth.VerifyAuthSecret(authSecret, storedHash); err != nil {
		t.Fatalf("auth secret verification failed: %v", err)
	}

	// 2. Second run is a no-op
	_ = os.Remove(passFile)
	if err := EnsureBootstrapAdmin(store.DB(), cfg); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}
	if _, err := os.Stat(passFile); !os.IsNotExist(err) {
		t.Fatalf("first-run-password.txt should not be re-created on second run")
	}
}

func TestEnsureBootstrapAdminCustomPass(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "kynotes.sqlite"))
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	cfg := config.Defaults()
	cfg.DataDir = dir
	cfg.Secrets.ServerSaltKey = "test-salt-key-32-bytes-long-1234"

	t.Setenv("BOOTSTRAP_ADMIN_USER", "custom_admin")
	t.Setenv("BOOTSTRAP_ADMIN_PASS", "SuperSecretPass123!")

	if err := EnsureBootstrapAdmin(store.DB(), cfg); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// first-run-password.txt should NOT be written when password was explicitly supplied
	passFile := filepath.Join(dir, "first-run-password.txt")
	if _, err := os.Stat(passFile); !os.IsNotExist(err) {
		t.Fatalf("first-run-password.txt should not be created when BOOTSTRAP_ADMIN_PASS is set")
	}

	var username, storedHash, salt string
	var iterations int
	err = store.DB().QueryRow(`SELECT username, auth_secret_hash, login_salt, login_iterations FROM users WHERE username='custom_admin'`).Scan(
		&username, &storedHash, &salt, &iterations,
	)
	if err != nil {
		t.Fatalf("custom_admin user not found in DB: %v", err)
	}

	authSecret, err := auth.DeriveAuthSecret("SuperSecretPass123!", salt, iterations)
	if err != nil {
		t.Fatalf("failed to derive auth secret: %v", err)
	}
	if err := auth.VerifyAuthSecret(authSecret, storedHash); err != nil {
		t.Fatalf("auth secret verification failed: %v", err)
	}
}
