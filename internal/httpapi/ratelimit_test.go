package httpapi

import (
	"testing"
	"time"
)

func TestLimiterConsumesAndRefillsTokens(t *testing.T) {
	l := newLimiter()
	now := time.Unix(100, 0)
	if !l.allow("key", 1, 2, now) || !l.allow("key", 1, 2, now) {
		t.Fatal("initial burst was not available")
	}
	if l.allow("key", 1, 2, now) {
		t.Fatal("third token exceeded burst")
	}
	if !l.allow("key", 1, 2, now.Add(time.Second)) {
		t.Fatal("token did not refill")
	}
}
