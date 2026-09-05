package auth

import (
	"strings"
	"testing"
)

func TestRecoveryCodeRoundTrip(t *testing.T) {
	code, hash, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 14 { // xxxx-xxxx-xxxx
		t.Fatalf("unexpected format %q", code)
	}
	if strings.HasPrefix(hash, code) || strings.Contains(hash, code) {
		t.Fatal("code stored in the clear")
	}
	if err := VerifyRecoveryCode(code, hash); err != nil {
		t.Fatal(err)
	}
	// Operators type codes with spaces and capitals.
	if err := VerifyRecoveryCode(" "+strings.ToUpper(code)+" ", hash); err != nil {
		t.Fatalf("normalised code must verify: %v", err)
	}
	if err := VerifyRecoveryCode("0000-0000-0000", hash); err == nil {
		t.Fatal("wrong code verified")
	}
}
