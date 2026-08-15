package blobstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, s *Store, id string, data []byte) string {
	t.Helper()
	x, err := s.NewTemp(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.Write(data); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	_ = x.Close()
	return hex.EncodeToString(sum[:])
}

func TestFinalizeIsIdempotentForDuplicateContent(t *testing.T) {
	s, _ := New(t.TempDir())
	data := []byte("same")
	d := writeTemp(t, s, "a", data)
	x, _ := s.NewTemp("b")
	_, _ = x.Write(data)
	if _, _, err := x.Finalize(d); err != nil {
		t.Fatal(err)
	}
	y, _ := s.NewTemp("c")
	_, _ = y.Write(data)
	if _, _, err := y.Finalize(d); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeRefusesToOverwriteDifferentSize(t *testing.T) {
	root := t.TempDir()
	s, _ := New(root)
	data := []byte("correct")
	sum := sha256.Sum256(data)
	d := hex.EncodeToString(sum[:])
	path := s.path(d)
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	if err := os.WriteFile(path, []byte("wrong-size"), 0600); err != nil {
		t.Fatal(err)
	}
	x, _ := s.NewTemp("wrong")
	_, _ = x.Write(data)
	if _, _, err := x.Finalize(d); !errors.Is(err, ErrCorruptBlob) {
		t.Fatalf("got %v", err)
	}
}

func TestAbortRemovesTempFile(t *testing.T) {
	s, _ := New(t.TempDir())
	x, _ := s.NewTemp("abort")
	if err := x.Abort(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.root, "tmp", "abort.part")); !os.IsNotExist(err) {
		t.Fatal("temp file remains")
	}
}

func TestAbortIsSafeTwice(t *testing.T) {
	s, _ := New(t.TempDir())
	x, _ := s.NewTemp("abort-twice")
	_ = x.Abort()
	_ = x.Abort()
}

func TestReopenRehashesPartialFile(t *testing.T) {
	s, _ := New(t.TempDir())
	x, _ := s.NewTemp("partial")
	_, _ = x.Write([]byte("partial"))
	_ = x.Close()
	y, err := s.Reopen("partial")
	if err != nil {
		t.Fatal(err)
	}
	defer y.Close()
	if y.Size() != 7 || y.Digest() != hex.EncodeToString(func() []byte { x := sha256.Sum256([]byte("partial")); return x[:] }()) {
		t.Fatal("reopen did not restore hash and size")
	}
}

func TestOpenMissingBlobReportsNotFound(t *testing.T) {
	s, _ := New(t.TempDir())
	if _, _, err := s.Open(strings.Repeat("0", 64)); !os.IsNotExist(err) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteMissingBlobIsNotAnError(t *testing.T) {
	s, _ := New(t.TempDir())
	if err := s.Delete(strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
}

func TestFileModesAre0600AndDirs0700(t *testing.T) {
	root := t.TempDir()
	s, _ := New(root)
	for _, dir := range []string{filepath.Join(root, "blobs"), filepath.Join(root, "tmp")} {
		info, _ := os.Stat(dir)
		if info.Mode().Perm() != 0700 {
			t.Fatalf("%s mode %o", dir, info.Mode().Perm())
		}
	}
	x, _ := s.NewTemp("mode")
	info, _ := os.Stat(filepath.Join(root, "tmp", "mode.part"))
	_ = x.Close()
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode %o", info.Mode().Perm())
	}
}

func TestPathShardingSplitsOnFirstFourHexChars(t *testing.T) {
	s, _ := New(t.TempDir())
	d := strings.Repeat("a", 64)
	if got := s.path(d); got != filepath.Join(s.root, "blobs", "aa", "aa", d) {
		t.Fatalf("path %s", got)
	}
}

func TestOpenRoundTripUsesExactBytes(t *testing.T) {
	s, _ := New(t.TempDir())
	data := []byte("round-trip")
	x, _ := s.NewTemp("round")
	_, _ = x.Write(data)
	sum := sha256.Sum256(data)
	d, _, _ := x.Finalize(hex.EncodeToString(sum[:]))
	f, _, err := s.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	_ = f.Close()
	if string(got) != string(data) {
		t.Fatalf("got %q", got)
	}
}
