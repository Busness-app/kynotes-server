package auth

import (
	"sync"
	"time"
)

type lockEntry struct {
	Failures    int
	LockedUntil time.Time
	Last        time.Time
}
type Lockout struct {
	mu         sync.Mutex
	items      map[string]lockEntry
	max, limit int
	cooldown   time.Duration
}

func NewLockout(limit int, cooldown time.Duration, max int) *Lockout {
	return &Lockout{items: make(map[string]lockEntry), limit: limit, cooldown: cooldown, max: max}
}
func (l *Lockout) Try(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.items[key]
	if now.Before(e.LockedUntil) {
		return false
	}
	if e.LockedUntil.IsZero() && e.Failures >= l.limit {
		e.LockedUntil = now.Add(l.cooldown)
		l.items[key] = e
		return false
	}
	if e.Failures >= l.limit {
		delete(l.items, key)
		return true
	}
	if _, ok := l.items[key]; !ok && len(l.items) >= l.max {
		return false
	}
	return true
}
func (l *Lockout) Fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.items[key]
	e.Failures++
	e.Last = now
	if e.Failures >= l.limit {
		e.LockedUntil = now.Add(l.cooldown)
	}
	l.items[key] = e
}
func (l *Lockout) Success(key string) { l.mu.Lock(); delete(l.items, key); l.mu.Unlock() }
func (l *Lockout) Sweep(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, e := range l.items {
		if e.LockedUntil.IsZero() && now.Sub(e.Last) > time.Hour {
			delete(l.items, k)
		}
	}
}
