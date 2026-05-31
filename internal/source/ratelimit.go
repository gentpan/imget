package source

import (
	"context"
	"sync"
	"time"
)

// Limiter enforces a minimum interval between upstream calls plus a post-429
// cooldown window. It was extracted from the byte-for-byte identical throttling
// code that lived in both the Pexels and Pixabay providers.
//
// Reserve-then-sleep: each caller advances nextFree under the lock *before*
// releasing it, so concurrent callers serialize correctly — the next caller
// sees the already-advanced timestamp and queues behind it.
type Limiter struct {
	minInterval time.Duration
	cooldown    time.Duration

	mu        sync.Mutex
	nextFree  time.Time
	coolUntil time.Time
}

// NewLimiter builds a Limiter. Zero values fall back to sensible defaults
// (1s interval, 5m cooldown) so callers can pass through unset config.
func NewLimiter(minIntervalMS, cooldownSec int) *Limiter {
	if minIntervalMS <= 0 {
		minIntervalMS = 1000
	}
	if cooldownSec <= 0 {
		cooldownSec = 300
	}
	return &Limiter{
		minInterval: time.Duration(minIntervalMS) * time.Millisecond,
		cooldown:    time.Duration(cooldownSec) * time.Second,
	}
}

// Wait blocks until the caller's reserved slot, honoring both the min interval
// and any active cooldown. It returns early if ctx is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	at := now
	if l.coolUntil.After(at) {
		at = l.coolUntil
	}
	if l.nextFree.After(at) {
		at = l.nextFree
	}
	l.nextFree = at.Add(l.minInterval)
	sleep := at.Sub(now)
	l.mu.Unlock()

	if sleep <= 0 {
		return nil
	}
	t := time.NewTimer(sleep)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// MarkCooldown opens a cooldown window (called after an upstream 429).
func (l *Limiter) MarkCooldown() {
	l.mu.Lock()
	l.coolUntil = time.Now().Add(l.cooldown)
	l.mu.Unlock()
}
