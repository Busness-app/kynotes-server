package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// LockDirectory gives server and offline recovery commands the same exclusive owner.
func LockDirectory(dir string) (func(), error) {
	path := filepath.Join(dir, ".kynotes.lock")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("data directory is already in use or inaccessible: %w", err)
	}
	if _, err = file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		file.Close()
		os.Remove(path)
		return nil, err
	}
	if err = file.Close(); err != nil {
		os.Remove(path)
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}
