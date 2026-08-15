package auth

import (
	"encoding/base64"
	"testing"
	"time"
)

func FuzzParsePairingToken(f *testing.F) {
	f.Add("bad")
	f.Fuzz(func(t *testing.T, token string) {
		_, _ = ParsePairingToken("12345678901234567890123456789012", token, "usr_test", time.Unix(1, 0))
	})
}

func FuzzParseAuthSecret(f *testing.F) {
	f.Add(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	f.Fuzz(func(t *testing.T, salt string) {
		_, _ = DeriveAuthSecret("fuzz-password", salt, MinLoginIterations)
	})
}
