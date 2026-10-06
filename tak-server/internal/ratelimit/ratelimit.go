// Package ratelimit keeps a token bucket per key, such as a client IP or a
// username.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Keyed allows up to a burst of events per key, refilling at a steady rate.
// It's safe for concurrent use.
type Keyed struct {
	limit   rate.Limit
	burst   int
	maxKeys int
	now     func() time.Time

	mu      sync.Mutex
	buckets map[string]*rate.Limiter
}

// New allows events per period for each key, all at once if they come in a
// burst. At most maxKeys keys are tracked; when that many are busy, new keys
// are refused rather than letting a flood of distinct keys use up memory.
func New(events int, per time.Duration, maxKeys int) *Keyed {
	return &Keyed{
		limit:   rate.Limit(float64(events) / per.Seconds()),
		burst:   events,
		maxKeys: maxKeys,
		now:     time.Now,
		buckets: make(map[string]*rate.Limiter),
	}
}

// Allow takes one event for key. If there's none left it returns false and
// how long until there will be.
func (k *Keyed) Allow(key string) (bool, time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	b, ok := k.bucketLocked(key)
	if !ok {
		return false, k.refill()
	}
	now := k.now()
	r := b.ReserveN(now, 1)
	if d := r.DelayFrom(now); d > 0 {
		r.CancelAt(now)
		return false, d
	}
	return true, 0
}

// Blocked reports, without taking one, whether key has no events left, and
// if so how long until it will. It's for counting failures: check Blocked
// before an attempt and Record after one fails.
func (k *Keyed) Blocked(key string) (bool, time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	b, ok := k.buckets[key]
	if !ok {
		return false, 0
	}
	if tokens := b.TokensAt(k.now()); tokens < 1 {
		return true, time.Duration((1 - tokens) / float64(k.limit) * float64(time.Second))
	}
	return false, 0
}

// Record uses up one event for key, even if none are left.
func (k *Keyed) Record(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if b, ok := k.bucketLocked(key); ok {
		b.ReserveN(k.now(), 1)
	}
}

// Reset forgets key, e.g. after a successful login.
func (k *Keyed) Reset(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.buckets, key)
}

// Prune forgets keys whose buckets have refilled, which behave the same as
// keys never seen.
func (k *Keyed) Prune() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.pruneLocked()
}

// Len reports how many keys are tracked.
func (k *Keyed) Len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.buckets)
}

func (k *Keyed) pruneLocked() {
	now := k.now()
	for key, b := range k.buckets {
		if b.TokensAt(now) >= float64(k.burst) {
			delete(k.buckets, key)
		}
	}
}

func (k *Keyed) bucketLocked(key string) (*rate.Limiter, bool) {
	if b, ok := k.buckets[key]; ok {
		return b, true
	}
	if len(k.buckets) >= k.maxKeys {
		k.pruneLocked()
		if len(k.buckets) >= k.maxKeys {
			return nil, false
		}
	}
	b := rate.NewLimiter(k.limit, k.burst)
	k.buckets[key] = b
	return b, true
}

// refill is how long one event takes to come back.
func (k *Keyed) refill() time.Duration {
	return time.Duration(float64(time.Second) / float64(k.limit))
}
