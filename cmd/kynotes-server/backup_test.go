package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busness-app/ky-primitives/recoveryclient"
	"github.com/Busness-app/ky-primitives/recoverykey"
	"github.com/Busness-app/kynotes-server/internal/auth"
	"github.com/Busness-app/kynotes-server/internal/backup"
	"github.com/Busness-app/kynotes-server/internal/config"
	"github.com/Busness-app/kynotes-server/internal/storage"
)

func TestRestoreCapsuleWithStdinSharesPreservesLoginAndRevokesSessions(t *testing.T) {
	cfg := config.Defaults()
	cfg.DataDir = t.TempDir()
	cfg.Secrets.PairingSecret = strings.Repeat("p", 32)
	cfg.Secrets.ServerSaltKey = strings.Repeat("s", 32)
	st, err := storage.Open(filepath.Join(cfg.DataDir, "kynotes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	secret := strings.Repeat("a", 64)
	hash, err := auth.HashAuthSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(`INSERT INTO users(id,username,auth_secret_hash,login_salt,login_iterations,role,status,created_at,updated_at) VALUES('usr_fixture','admin',?,'salt',600000,'admin','active','','')`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(`INSERT INTO sessions(id,user_id,token_hash,csrf_hash,created_at,expires_at,hard_expires_at) VALUES('sess_fixture','usr_fixture','token','csrf',?,?,?)`, time.Now().Format(time.RFC3339), time.Now().Add(time.Hour).Format(time.RFC3339), time.Now().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	svc := backup.New(cfg, st, "test")
	key, err := recoverykey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Pin("system", "fixture", base64.StdEncoding.EncodeToString(key.Public().Bytes()), 2, 3); err != nil {
		t.Fatal(err)
	}
	raw, _, err := svc.Export(context.Background(), "system", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.kycap")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	shares, err := recoverykey.Split(key, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	input := shares[0].String() + "\n" + shares[2].String() + "\n"
	parent, err := os.MkdirTemp(".", ".restore-proof-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(parent) })
	parent, err = filepath.Abs(parent)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "restored")
	var output bytes.Buffer
	if err = restoreCapsule(path, target, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), shares[0].String()) || strings.Contains(output.String(), cfg.Secrets.ServerSaltKey) {
		t.Fatal("restore printed secrets")
	}
	got, err := config.Load(filepath.Join(target, "kynotes.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got.DataDir != target || got.Secrets != cfg.Secrets {
		t.Fatal("restore changed secrets or directory")
	}
	restored, err := storage.Open(filepath.Join(target, "kynotes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var verifier, revoked string
	if err = restored.DB().QueryRow(`SELECT auth_secret_hash FROM users WHERE id='usr_fixture'`).Scan(&verifier); err != nil {
		t.Fatal(err)
	}
	if err = auth.VerifyAuthSecret(secret, verifier); err != nil {
		t.Fatal("restored login failed")
	}
	if err = restored.DB().QueryRow(`SELECT revoked_at FROM sessions WHERE id='sess_fixture'`).Scan(&revoked); err != nil || revoked == "" {
		t.Fatal("old session resurrected")
	}
	if err = restoreCapsule(path, target, strings.NewReader(input), &output); err == nil {
		t.Fatal("overwrote occupied target")
	}
	if err = restoreCapsule(path, filepath.Join(t.TempDir(), "bad"), strings.NewReader(shares[0].String()), &output); err == nil {
		t.Fatal("accepted insufficient shares")
	}
	bad := append([]byte{}, raw...)
	bad[len(bad)/2] ^= 1
	if err = os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err = restoreCapsule(path, filepath.Join(t.TempDir(), "tampered"), strings.NewReader(input), &output); err == nil {
		t.Fatal("accepted tampered capsule")
	}
	wrong, _, err := recoveryclient.Seal(recoveryclient.Payload{ServiceName: "Other", AppVersion: "test", Files: []recoveryclient.File{{Path: "file", Data: []byte("x"), Mode: 0600}}}, recoveryclient.RecoveryKey{Public: key.Public(), Threshold: 2, TotalShares: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, wrong, 0600); err != nil {
		t.Fatal(err)
	}
	if err = restoreCapsule(path, filepath.Join(t.TempDir(), "foreign"), strings.NewReader(input), &output); err == nil {
		t.Fatal("foreign service accepted")
	}
}
