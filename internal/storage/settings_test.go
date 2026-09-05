package storage

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "k.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.GetSetting("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.SetSetting("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting("a", "2"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetSetting("a"); v != "2" {
		t.Fatalf("got %q", v)
	}
	if err := s.SetSetting("empty", ""); err != nil {
		t.Fatal(err)
	}
	if v, err := s.GetSetting("empty"); err != nil || v != "" {
		t.Fatalf("a key set to empty is present, not missing: %v %q", err, v)
	}
	if err := s.DeleteSetting("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSetting("a"); err != nil {
		t.Fatalf("deleting an absent key must be a no-op: %v", err)
	}
	if _, err := s.GetSetting("a"); !errors.Is(err, ErrNotFound) {
		t.Fatal("still present after delete")
	}
}
