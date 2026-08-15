package auth

import (
	"testing"
	"time"
)

func TestPairingTokenRoundTripAndExpiry(t *testing.T) {
	now := time.Unix(1000, 0)
	tok, c, e := MintPairingToken("12345678901234567890123456789012", "usr_x", now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ParsePairingToken("12345678901234567890123456789012", tok, "usr_x", now); e != nil {
		t.Fatal(e)
	}
	if _, e = ParsePairingToken("12345678901234567890123456789012", tok, "usr_x", now.Add(121*time.Second)); e == nil {
		t.Fatal("expired token accepted")
	}
	if _, e = ParsePairingToken("bad", tok, "usr_x", now); e == nil {
		t.Fatal("bad secret accepted")
	}
	_ = c
}
