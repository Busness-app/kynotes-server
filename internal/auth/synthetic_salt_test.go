package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// The salt is a contract with every client that already learned one: it must be
// byte-identical to the formula this repo shipped before ky-primitives/derive.
func TestSyntheticLoginSaltMatchesPreLibraryFormula(t *testing.T) {
	key := strings.Repeat("k", 32)
	for _, u := range []string{"alice", "Alice", "  Bob"} {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte("login-salt/v1\x00" + strings.ToLower(u)))
		want := base64.StdEncoding.EncodeToString(mac.Sum(nil)[:16])
		if got := SyntheticLoginSalt(key, u); got != want {
			t.Fatalf("%q: got %s want %s", u, got, want)
		}
	}
}

func TestSyntheticLoginSaltIsNeverEmptyForAValidKey(t *testing.T) {
	if SyntheticLoginSalt(strings.Repeat("k", 32), "x") == "" {
		t.Fatal("empty salt")
	}
}
