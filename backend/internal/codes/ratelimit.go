package codes

import (
	"sync"
	"time"
)

// AttemptLimiter is a simple in-memory sliding window for brute-force protection.
// Suitable for single-node lab/dev; replace with Redis for multi-instance prod.
type AttemptLimiter struct {
	mu        sync.Mutex
	attempts  map[string][]time.Time
	window    time.Duration
	max       int
	lastSweep time.Time
}

// NewAttemptLimiter allows at most max failed attempts per key within window.
// Once a key has recorded max failures, further attempts are denied until the
// oldest failure ages out of the window.
func NewAttemptLimiter(max int, window time.Duration) *AttemptLimiter {
	if max < 1 {
		max = 10
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &AttemptLimiter{
		attempts:  make(map[string][]time.Time),
		window:    window,
		max:       max,
		lastSweep: time.Now(),
	}
}

// Allow reports whether an attempt for key may proceed.
// failedAttempt=true records a failure (use after an invalid try).
//
// Semantics: an attempt is denied when the key already has max failures in
// the current window, so at most max failures can ever be recorded per window.
func (l *AttemptLimiter) Allow(key string, failedAttempt bool) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Periodic full sweep so keys that stop failing don't live forever.
	if now.Sub(l.lastSweep) > l.window {
		l.sweep(now)
	}

	arr := l.attempts[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	blocked := len(kept) >= l.max
	if !blocked && failedAttempt {
		kept = append(kept, now)
	}
	if len(kept) == 0 {
		delete(l.attempts, key)
	} else {
		l.attempts[key] = kept
	}

	return !blocked
}

func (l *AttemptLimiter) sweep(now time.Time) {
	l.lastSweep = now
	cutoff := now.Add(-l.window)
	for k, arr := range l.attempts {
		kept := arr[:0]
		for _, t := range arr {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.attempts, k)
		} else {
			l.attempts[k] = kept
		}
	}
}
