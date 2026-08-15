package auth

import (
	"testing"
	"time"
)

func TestLockout(t *testing.T) {
	l := NewLockout(3, time.Minute, 10)
	now := time.Unix(0, 0)
	for i := 0; i < 3; i++ {
		if !l.Try("x", now) {
			t.Fatal("unexpected lock")
		}
		l.Fail("x", now)
	}
	if l.Try("x", now) {
		t.Fatal("lockout not enforced")
	}
	if l.Try("x", now.Add(30*time.Second)) {
		t.Fatal("cooldown not enforced")
	}
	if !l.Try("x", now.Add(2*time.Minute)) {
		t.Fatal("expired cooldown not reopened")
	}
	l.Success("x")
	if !l.Try("x", now) {
		t.Fatal("success did not clear")
	}
}
