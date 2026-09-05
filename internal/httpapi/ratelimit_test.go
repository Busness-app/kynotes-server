package httpapi

import (
	"net/http/httptest"
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

func TestRateLimitForwardedIdentity(t *testing.T) {
	proxies := parseTrustedProxies([]string{"127.0.0.1/32", "10.0.0.0/8"})
	for _, tc := range []struct {
		name, peer, forwarded, want string
		behind                      bool
	}{
		{"direct", "192.0.2.1:1234", "192.0.2.9", "192.0.2.1", true},
		{"disabled", "127.0.0.1:1234", "192.0.2.9", "127.0.0.1", false},
		{"trusted", "127.0.0.1:1234", "192.0.2.9", "192.0.2.9", true},
		{"spoofed prefix", "127.0.0.1:1234", "192.0.2.99, 192.0.2.9, 10.0.0.2", "192.0.2.9", true},
		{"invalid suffix", "127.0.0.1:1234", "192.0.2.9, invalid", "127.0.0.1", true},
		{"missing", "127.0.0.1:1234", "", "127.0.0.1", true},
		{"all trusted", "127.0.0.1:1234", "10.0.0.2", "127.0.0.1", true},
		{"ipv6 subnet", "127.0.0.1:1234", "2001:db8:abcd:1234::99", "2001:db8:abcd:1234::", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := rateLimitClientIP(r, tc.behind, proxies); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Add("X-Forwarded-For", "192.0.2.99")
	r.Header.Add("X-Forwarded-For", "192.0.2.9, 10.0.0.2")
	if got := rateLimitClientIP(r, true, proxies); got != "192.0.2.9" {
		t.Fatal("multiple header lines changed trust order", got)
	}
}
