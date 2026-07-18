package codes

import (
	"sync"
	"time"
)

// AttemptLimiter is a simple in-memory sliding window for brute-force protection.
// Suitable for single-node lab/dev; replace with Redis for multi-instance prod.
type AttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	max      int
}

// NewAttemptLimiter allows max events per key within window.
func NewAttemptLimiter(max int, window time.Duration) *AttemptLimiter {
	if max < 1 {
		max = 10
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &AttemptLimiter{
		attempts: make(map[string][]time.Time),
		window:   window,
		max:      max,
	}
}

// Allow returns false when the key has exceeded the budget.
// failedAttempt=true records a hit (use for invalid code tries).
func (l *AttemptLimiter) Allow(key string, failedAttempt bool) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	arr := l.attempts[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if failedAttempt {
		kept = append(kept, now)
	}
	l.attempts[key] = kept

	// Count failed attempts in window; block when at/over max.
	return len(kept) <= l.max
}
