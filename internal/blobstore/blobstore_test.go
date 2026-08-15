package blobstore

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeRoundTrip(t *testing.T) {
	root := t.TempDir()
	s, e := New(root)
	if e != nil {
		t.Fatal(e)
	}
	x, e := s.NewTemp("ups_test")
	if e != nil {
		t.Fatal(e)
	}
	data := []byte("ciphertext")
	if _, e = x.Write(data); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(data)
	d, n, e := x.Finalize(hex.EncodeToString(sum[:]))
	if e != nil || n != int64(len(data)) {
		t.Fatalf("finalize: %v %d", e, n)
	}
	f, n, e := s.Open(d)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	got := make([]byte, n)
	if _, e = f.Read(got); e != nil {
		t.Fatal(e)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q", got)
	}
}
func TestFinalizeRejectsDigestMismatch(t *testing.T) {
	s, _ := New(t.TempDir())
	x, _ := s.NewTemp("x")
	_, _ = x.Write([]byte("x"))
	if _, _, e := x.Finalize("00"); e != ErrDigestMismatch {
		t.Fatalf("got %v", e)
	}
	if _, e := os.Stat(filepath.Join(s.root, "tmp", "x.part")); !os.IsNotExist(e) {
		t.Fatal("temp remains")
	}
}
