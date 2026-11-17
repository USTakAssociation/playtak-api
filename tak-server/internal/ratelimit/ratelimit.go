// Package ratelimit keeps a token bucket per key, such as a client IP or a
// username.
package ratelimit

import (
	"net/netip"
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

	mu         sync.Mutex
	buckets    map[string]*rate.Limiter
	inProgress map[string]int // attempts started but not finished, per key
}

// New allows events per period for each key, all at once if they come in a
// burst. At most maxKeys keys are tracked; when that many are busy, new keys
// are refused rather than letting a flood of distinct keys use up memory.
func New(events int, per time.Duration, maxKeys int) *Keyed {
	return &Keyed{
		limit:      rate.Limit(float64(events) / per.Seconds()),
		burst:      events,
		maxKeys:    maxKeys,
		now:        time.Now,
		buckets:    make(map[string]*rate.Limiter),
		inProgress: make(map[string]int),
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

// Attempt starts an attempt for key that uses up an event only if it fails,
// for limits that count failures. Attempts still in progress count as
// failures, so many at once can't all get past the limit before the first
// one fails. If key has no events left, finish is nil and wait says how long
// until it will; otherwise call finish exactly once with the outcome.
func (k *Keyed) Attempt(key string) (finish func(failed bool), wait time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	tokens := float64(k.burst)
	if b, ok := k.buckets[key]; ok {
		tokens = b.TokensAt(k.now())
	} else if len(k.buckets) >= k.maxKeys {
		// A full table can't record this key's failures, so refusing is the
		// only way to keep the limit; otherwise a flood of keys switches it off.
		k.pruneLocked()
		if len(k.buckets) >= k.maxKeys {
			return nil, k.refill()
		}
	}
	if left := tokens - float64(k.inProgress[key]); left < 1 {
		return nil, time.Duration((1 - left) / float64(k.limit) * float64(time.Second))
	}
	k.inProgress[key]++
	return func(failed bool) {
		k.mu.Lock()
		defer k.mu.Unlock()
		if k.inProgress[key]--; k.inProgress[key] <= 0 {
			delete(k.inProgress, key)
		}
		if !failed {
			return
		}
		if b, ok := k.bucketLocked(key); ok {
			b.ReserveN(k.now(), 1) // may go below zero, pushing the wait out
		}
	}, 0
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

// IPKey is the key to limit ip by. An IPv6 address is limited by its /64,
// since one machine or home network usually gets a whole /64. Anything that
// isn't an IP, such as "", is returned unchanged.
func IPKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is6() || addr.Is4In6() {
		return ip
	}
	prefix, _ := addr.Prefix(64)
	return prefix.String()
}

// refill is how long one event takes to come back.
func (k *Keyed) refill() time.Duration {
	return time.Duration(float64(time.Second) / float64(k.limit))
}
