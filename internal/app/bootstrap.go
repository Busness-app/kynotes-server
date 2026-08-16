package app

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yoshiofthewire/kynotes-server/internal/auth"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/ids"
)

// EnsureBootstrapAdmin seeds initial admin credentials if no users exist in the database.
func EnsureBootstrapAdmin(db *sql.DB, c config.Config) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	username := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USER"))
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("BOOTSTRAP_ADMIN_PASS")
	generated := password == ""
	if generated {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			return fmt.Errorf("failed to generate bootstrap password: %w", err)
		}
		password = base64.RawURLEncoding.EncodeToString(b)
	}

	salt := auth.SyntheticLoginSalt(c.Secrets.ServerSaltKey, username)
	iterations := 600000
	authSecret, err := auth.DeriveAuthSecret(password, salt, iterations)
	if err != nil {
		return fmt.Errorf("failed to derive bootstrap auth secret: %w", err)
	}

	hash, err := auth.HashAuthSecret(authSecret)
	if err != nil {
		return fmt.Errorf("failed to hash bootstrap auth secret: %w", err)
	}

	id, err := ids.Mint("usr")
	if err != nil {
		return fmt.Errorf("failed to mint admin user id: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO users(id, username, auth_secret_hash, login_salt, login_iterations, role, status, created_at, updated_at) VALUES(?, ?, ?, ?, ?, 'admin', 'active', ?, ?)`,
		id, strings.ToLower(username), hash, salt, iterations, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert bootstrap admin user: %w", err)
	}

	if generated {
		passFile := filepath.Join(c.DataDir, "first-run-password.txt")
		content := fmt.Sprintf("User: %s\nPassword: %s\n", username, password)
		if err := os.WriteFile(passFile, []byte(content), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write bootstrap password to %s: %v\n", passFile, err)
		} else {
			fmt.Printf("Initial admin account created. Credentials written to %s (read and delete this file).\n", passFile)
		}
		fmt.Printf("\n==================================================\n  KYNOTES FIRST-RUN ADMIN CREDENTIALS\n  Username: %s\n  Password: %s\n==================================================\n\n", username, password)
	} else {
		fmt.Printf("Initial admin account created for '%s' from BOOTSTRAP_ADMIN_PASS.\n", username)
	}

	return nil
}
