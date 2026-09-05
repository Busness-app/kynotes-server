package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlobTargetValidationAndEnv(t *testing.T) {
	for _, c := range []BlobTarget{{}, {URL: "file:///mnt/ciphertext"}, {URL: "s3://bucket/prefix", AccessKey: "id", Secret: "secret"}, {URL: "sftp://user@host/dir", Secret: "password", HostKey: "SHA256:verified"}} {
		if err := ValidateBlobTarget(c); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []BlobTarget{{URL: "ftp://host"}, {URL: "s3://bucket"}, {URL: "sftp://user@host/dir", Secret: "password"}, {URL: "sftp://user:DO_NOT_LEAK@host/dir", HostKey: "pin"}, {URL: "file://host/dir"}} {
		err := ValidateBlobTarget(c)
		if err == nil || strings.Contains(err.Error(), "DO_NOT_LEAK") {
			t.Fatal("invalid target accepted or secret leaked")
		}
	}
	t.Setenv("KYNOTES_BLOB_TARGET", "file:///override")
	t.Setenv("KYNOTES_BLOB_TARGET_SECRET", "")
	c := BlobTarget{URL: "file:///original"}
	applyBlobEnv(&c)
	if c.URL != "file:///override" {
		t.Fatal("env not applied")
	}
}

func TestOnlyProbeMayLoadUnpinnedSFTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := "data_dir: " + t.TempDir() + "\nserver:\n  bind: 127.0.0.1:8080\n  dev_insecure_cookies: true\nbackup:\n  blob_target:\n    url: sftp://fixture@localhost/vault\n    secret: synthetic\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("production accepted missing host pin")
	}
	cfg, err := LoadBlobTargetProbe(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backup.BlobTarget.HostKey != "" {
		t.Fatal("probe invented trust")
	}
}
