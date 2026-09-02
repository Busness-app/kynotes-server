package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"database/sql"
	"github.com/Busness-app/kynotes-server/internal/auth"
	"github.com/Busness-app/kynotes-server/internal/config"
)

type bucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
}

func newLimiter() *limiter { return &limiter{buckets: make(map[string]bucket)} }

func (l *limiter) allow(key string, rate float64, burst int, now time.Time) bool {
	if rate <= 0 || burst <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b.last.IsZero() {
		b.tokens = float64(burst)
		b.last = now
	}
	b.tokens += now.Sub(b.last).Seconds() * float64(rate)
	if b.tokens > float64(burst) {
		b.tokens = float64(burst)
	}
	b.last = now
	if b.tokens < 1 {
		l.buckets[key] = b
		return false
	}
	b.tokens--
	l.buckets[key] = b
	return true
}

func rateLimitMiddleware(cfg config.Config, db *sql.DB, next http.Handler) http.Handler {
	l := newLimiter()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, rate, label := 0, 0, ""
		switch {
		case r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/login-params":
			limit, rate, label = cfg.RateLimit.LoginPerMinute, cfg.RateLimit.LoginPerMinute, "login"
		case r.URL.Path == "/api/v1/devices/pairing-token":
			limit, rate, label = cfg.RateLimit.PairingPerHour, cfg.RateLimit.PairingPerHour, "pairing"
		case (strings.HasPrefix(r.URL.Path, "/api/v1/containers/") && strings.HasSuffix(r.URL.Path, "/uploads")) || strings.HasPrefix(r.URL.Path, "/api/v1/uploads/"):
			limit, rate, label = cfg.RateLimit.UploadPerMinute, cfg.RateLimit.UploadPerMinute, "upload"
		}
		refill := float64(rate) / 60
		if label == "pairing" {
			refill = float64(rate) / 3600
		}
		identity := clientIP(r)
		if label != "login" && db != nil {
			if s, err := auth.ResolveSession(db, r, time.Now().UTC()); err == nil {
				identity = s.UserID
			}
		}
		if label != "" && !l.allow(label+"\x00"+identity, refill, limit, time.Now().UTC()) {
			w.Header().Set("Retry-After", strconv.Itoa(60))
			WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
