package config

import (
	"strings"
	"testing"
)

func TestBackupConfigDefaults(t *testing.T) {
	c := testConfig(t)
	if c.Backup.Keep != 7 || c.Backup.DepositInterval != "24h" || c.Backup.AllowPrivateRecovery || c.Backup.Dir != "" {
		t.Fatalf("defaults: %+v", c.Backup)
	}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
}

func TestBackupIntervalFloorAndOff(t *testing.T) {
	c := testConfig(t)
	c.Backup.DepositInterval = "5m"
	if err := Validate(c); err == nil || !strings.Contains(err.Error(), "backup.deposit_interval") {
		t.Fatalf("interval under 15m must be refused: %v", err)
	}
	c.Backup.DepositInterval = "-1h"
	if err := Validate(c); err == nil {
		t.Fatal("negative interval accepted")
	}
	c.Backup.DepositInterval = "0"
	if err := Validate(c); err != nil {
		t.Fatalf("0 disables: %v", err)
	}
	c.Backup.DepositInterval = "15m"
	if err := Validate(c); err != nil {
		t.Fatalf("floor is inclusive: %v", err)
	}
}

func TestBackupKeepBelowOneIsRefused(t *testing.T) {
	c := testConfig(t)
	c.Backup.Keep = 0
	if err := Validate(c); err == nil || !strings.Contains(err.Error(), "backup.keep") {
		t.Fatalf("keep 0 would prune every copy: %v", err)
	}
}

func TestBackupEnvOverrides(t *testing.T) {
	t.Setenv("KYNOTES_BACKUP_DIR", "/backups")
	t.Setenv("KYNOTES_BACKUP_KEEP", "3")
	t.Setenv("KYNOTES_BACKUP_DEPOSIT_INTERVAL", "0")
	t.Setenv("KYNOTES_BACKUP_ALLOW_PRIVATE_RECOVERY", "true")
	c := Defaults()
	if err := applyEnv(&c); err != nil {
		t.Fatal(err)
	}
	if c.Backup.Dir != "/backups" || c.Backup.Keep != 3 || c.Backup.DepositInterval != "0" || !c.Backup.AllowPrivateRecovery {
		t.Fatalf("env not applied: %+v", c.Backup)
	}
	t.Setenv("KYNOTES_BACKUP_KEEP", "0")
	if err := applyEnv(&c); err == nil {
		t.Fatal("KYNOTES_BACKUP_KEEP=0 accepted")
	}
}
