package api

import (
	"sync"
	"time"
)

// loginLimiter throttles login attempts per key (username) with a sliding window.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

const (
	loginWindow = time.Minute
	loginMax    = 10
)

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string][]time.Time)}
}

// allow records an attempt and reports whether it is within the rate limit.
func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-loginWindow)
	recent := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= loginMax {
		l.attempts[key] = recent
		return false
	}
	l.attempts[key] = append(recent, time.Now())
	return true
}
