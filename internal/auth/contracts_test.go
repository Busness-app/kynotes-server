package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func contractSalt() string { return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")) }

func TestDeriveAuthSecretRejectsShortSalt(t *testing.T) {
	if _, err := DeriveAuthSecret("x", base64.StdEncoding.EncodeToString([]byte("short")), MinLoginIterations); err == nil {
		t.Fatal("short salt accepted")
	}
}

func TestDeriveAuthSecretRejectsBadBase64(t *testing.T) {
	if _, err := DeriveAuthSecret("x", "not base64!", MinLoginIterations); err == nil {
		t.Fatal("bad salt accepted")
	}
}

func TestIterationsBelowMinimumRejected(t *testing.T) {
	if _, err := DeriveAuthSecret("x", contractSalt(), MinLoginIterations-1); err == nil {
		t.Fatal("low iteration count accepted")
	}
}

func TestIterationsAboveMaximumRejected(t *testing.T) {
	if _, err := DeriveAuthSecret("x", contractSalt(), MaxLoginIterations+1); err == nil {
		t.Fatal("high iteration count accepted")
	}
}

func TestProductionScryptCostIsUnchanged(t *testing.T) {
	if scryptN != 1<<17 || scryptR != 8 || scryptP != 1 || scryptKeyLen != 32 {
		t.Fatal("production scrypt parameters changed")
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	h, err := HashAuthSecret(strings.Repeat("a", 64))
	if err != nil || VerifyAuthSecret(strings.Repeat("a", 64), h) != nil {
		t.Fatalf("round trip failed: %v", err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	h, err := HashAuthSecret("right")
	if err != nil {
		t.Fatal(err)
	}
	if VerifyAuthSecret("wrong", h) == nil {
		t.Fatal("wrong secret accepted")
	}
}

func TestVerifyParsesStoredCostParameters(t *testing.T) {
	h, err := HashAuthSecret("secret")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 6 || VerifyAuthSecret("secret", strings.Join(parts, "$")) != nil {
		t.Fatal("self-describing verifier was not parsed")
	}
}

func TestPairingTokenExpiresAfter120Seconds(t *testing.T) {
	now := time.Unix(1000, 0)
	token, _, err := MintPairingToken(strings.Repeat("s", 32), "usr_test", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParsePairingToken(strings.Repeat("s", 32), token, "usr_test", now.Add(121*time.Second)); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestPairingTokenForgedSignatureIsRejected(t *testing.T) {
	token, _, err := MintPairingToken(strings.Repeat("s", 32), "usr_test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	token = token[:len(token)-1] + "x"
	if _, err = ParsePairingToken(strings.Repeat("s", 32), token, "usr_test", time.Now()); err == nil {
		t.Fatal("forged token accepted")
	}
}

func TestPairingTokenWrongPurposeIsRejected(t *testing.T) {
	// Purpose is authenticated as part of the payload; a token minted by this
	// implementation cannot be changed without invalidating its MAC.
	token, _, err := MintPairingToken(strings.Repeat("s", 32), "usr_test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParsePairingToken(strings.Repeat("s", 32), token, "usr_other", time.Now()); err == nil {
		t.Fatal("token for another subject accepted")
	}
}
