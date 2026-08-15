package config

import (
	"os"
	"testing"
)

func FuzzConfigLoad(f *testing.F) {
	f.Add("/nonexistent")
	f.Fuzz(func(t *testing.T, path string) {
		if path == "" {
			return
		}
		_, _ = os.Stat(path)
	})
}
