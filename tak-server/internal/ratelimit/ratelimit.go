// Package ratelimit keeps a token bucket per key, such as a client IP or a
// username.
package ratelimit

import (
	"log/slog"
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
	uncounted  int            // events let through because the table was full

	lastFullPrune time.Time
}

// New allows events per period for each key, all at once if they come in a
// burst. At most maxKeys keys are tracked, so a flood of distinct keys can't
// use up memory. When that many are busy, events for new keys are let through
// uncounted rather than refused: refusing would lock out every new user.
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
		k.uncounted++
		return true, 0
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
	// Created now, not on failure, so the table can't fill up in between and
	// lose the failure.
	b, ok := k.bucketLocked(key)
	if !ok {
		k.uncounted++
		return func(bool) {}, 0
	}
	if left := b.TokensAt(k.now()) - float64(k.inProgress[key]); left < 1 {
		return nil, time.Duration((1 - left) / float64(k.limit) * float64(time.Second))
	}
	k.inProgress[key]++
	return func(failed bool) {
		k.mu.Lock()
		defer k.mu.Unlock()
		if k.inProgress[key]--; k.inProgress[key] <= 0 {
			delete(k.inProgress, key)
		}
		// Usually b, but Reset may have dropped it meanwhile.
		b, ok := k.bucketLocked(key)
		switch {
		case !ok:
			k.uncounted++
		case failed:
			b.ReserveN(k.now(), 1) // may go below zero, pushing the wait out
		case k.inProgress[key] == 0 && k.refilled(b):
			delete(k.buckets, key) // only successes: same as never seen
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
// keys never seen. If the table filled up since the last call, it warns on
// log, naming the limit what: that means a flood of distinct keys, which
// went uncounted.
func (k *Keyed) Prune(log *slog.Logger, what string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.pruneLocked()
	if k.uncounted > 0 {
		log.Warn("rate limit table full, events let through uncounted", "limit", what, "count", k.uncounted)
		k.uncounted = 0
	}
}

// Len reports how many keys are tracked.
func (k *Keyed) Len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.buckets)
}

// pruneLocked keeps keys with attempts in progress, so their failures land.
func (k *Keyed) pruneLocked() {
	for key, b := range k.buckets {
		if k.inProgress[key] == 0 && k.refilled(b) {
			delete(k.buckets, key)
		}
	}
}

func (k *Keyed) refilled(b *rate.Limiter) bool {
	return b.TokensAt(k.now()) >= float64(k.burst)
}

func (k *Keyed) bucketLocked(key string) (*rate.Limiter, bool) {
	if b, ok := k.buckets[key]; ok {
		return b, true
	}
	if len(k.buckets) >= k.maxKeys {
		// A prune scans every key, so while the table stays full it runs at
		// most once a second, or each request in a flood would pay for one.
		if now := k.now(); now.Sub(k.lastFullPrune) >= time.Second {
			k.lastFullPrune = now
			k.pruneLocked()
		}
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
