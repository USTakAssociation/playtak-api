package ratelimit

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Unix(1_800_000_000, 0)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// clocked returns a limiter on a fake clock and a function to move it on.
func clocked(events int, per time.Duration, maxKeys int) (*Keyed, func(time.Duration)) {
	k := New(events, per, maxKeys)
	now := t0
	var mu sync.Mutex
	k.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	return k, func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
}

func TestAllowBurstThenRefill(t *testing.T) {
	k, advance := clocked(5, time.Minute, 100)
	for i := range 5 {
		if ok, _ := k.Allow("ip"); !ok {
			t.Fatalf("event %d refused within the burst", i+1)
		}
	}
	ok, wait := k.Allow("ip")
	if ok || wait != 12*time.Second {
		t.Fatalf("6th event: ok=%v wait=%v, want refused for 12s", ok, wait)
	}
	// Other keys are unaffected.
	if ok, _ := k.Allow("other"); !ok {
		t.Error("keys must be independent")
	}
	advance(12 * time.Second)
	if ok, _ := k.Allow("ip"); !ok {
		t.Error("one event should have refilled")
	}
	if ok, _ := k.Allow("ip"); ok {
		t.Error("only one event refilled")
	}
}

func TestRefusedEventsDontCount(t *testing.T) {
	k, advance := clocked(1, time.Minute, 100)
	k.Allow("ip")
	for range 10 {
		k.Allow("ip") // hammering while limited mustn't push the wait back
	}
	advance(time.Minute)
	if ok, _ := k.Allow("ip"); !ok {
		t.Error("refused events must not use up future ones")
	}
}

// fail records one failed attempt for key.
func fail(k *Keyed, key string) {
	if finish, _ := k.Attempt(key); finish != nil {
		finish(true)
	}
}

// blocked reports whether an attempt for key would be refused, and the wait,
// without counting one.
func blocked(k *Keyed, key string) (bool, time.Duration) {
	finish, wait := k.Attempt(key)
	if finish != nil {
		finish(false)
	}
	return finish == nil, wait
}

func TestAttemptCountsFailures(t *testing.T) {
	k, advance := clocked(3, 15*time.Minute, 100)
	if b, _ := blocked(k, "alice"); b {
		t.Fatal("unknown key is not blocked")
	}
	for range 3 {
		fail(k, "alice")
	}
	b, wait := blocked(k, "alice")
	if !b || wait != 5*time.Minute {
		t.Fatalf("blocked=%v wait=%v, want blocked for 5m", b, wait)
	}
	advance(5 * time.Minute)
	fail(k, "alice") // uses the one that refilled
	if _, wait := blocked(k, "alice"); wait != 5*time.Minute {
		t.Errorf("wait %v, want 5m", wait)
	}
	advance(15 * time.Minute)
	if b, _ := blocked(k, "alice"); b {
		t.Error("should have refilled")
	}
	fail(k, "alice")
	fail(k, "alice")
	k.Reset("alice")
	if b, _ := blocked(k, "alice"); b || k.Len() != 0 {
		t.Error("Reset should forget the key")
	}
}

func TestAttemptSuccessDoesntCount(t *testing.T) {
	k, _ := clocked(2, time.Minute, 100)
	for range 10 {
		finish, _ := k.Attempt("ip")
		if finish == nil {
			t.Fatal("successes must not use up the limit")
		}
		finish(false)
	}
	if k.Len() != 0 {
		t.Errorf("%d keys tracked after only successes, want 0", k.Len())
	}
}

func TestAttemptsInProgressCount(t *testing.T) {
	k, _ := clocked(2, time.Minute, 100)
	first, _ := k.Attempt("ip")
	second, _ := k.Attempt("ip")
	if first == nil || second == nil {
		t.Fatal("two attempts fit in a burst of two")
	}
	// Both are still in progress, so a third can't start, even though
	// nothing has failed yet.
	if third, _ := k.Attempt("ip"); third != nil {
		t.Fatal("a third parallel attempt got past the limit")
	}
	first(false) // a success frees its slot
	third, _ := k.Attempt("ip")
	if third == nil {
		t.Fatal("a finished success should free its slot")
	}
	second(true)
	third(true)
	if b, _ := blocked(k, "ip"); !b {
		t.Error("two failures should block")
	}
}

func TestPrune(t *testing.T) {
	k, advance := clocked(2, time.Minute, 100)
	k.Allow("a")
	k.Allow("b")
	k.Allow("b")
	advance(30 * time.Second) // a is full again, b has one of two
	k.Prune(discard, "test")
	if k.Len() != 1 {
		t.Errorf("%d keys, want only b", k.Len())
	}
}

func TestFullTableLetsNewKeysThrough(t *testing.T) {
	k, advance := clocked(1, time.Minute, 2)
	k.Allow("a")
	k.Allow("b")
	// Refusing would lock out every new key, so it's let through uncounted.
	for range 3 {
		if ok, _ := k.Allow("c"); !ok {
			t.Fatal("a new key is refused while the table is full")
		}
	}
	if k.Len() != 2 {
		t.Errorf("%d keys tracked, want at most 2", k.Len())
	}
	if ok, _ := k.Allow("a"); ok {
		t.Error("keys already tracked stay limited")
	}
	var log strings.Builder
	k.Prune(slog.New(slog.NewTextHandler(&log, nil)), "test limit")
	if !strings.Contains(log.String(), "count=3") || !strings.Contains(log.String(), `limit="test limit"`) {
		t.Errorf("want a warning counting 3 uncounted events, got %q", log.String())
	}
	log.Reset()
	k.Prune(slog.New(slog.NewTextHandler(&log, nil)), "test limit")
	if log.Len() != 0 {
		t.Errorf("warned again with nothing new: %q", log.String())
	}
	advance(time.Minute) // a and b refill, so they can be pruned
	k.Allow("c")
	k.Allow("c")
	if ok, _ := k.Allow("c"); ok {
		t.Error("a new key is counted once old ones are pruned")
	}
}

func TestConcurrentUse(t *testing.T) {
	// The clock is frozen, so nothing refills: exactly the burst is allowed.
	k, _ := clocked(1000, time.Second, 1000)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for range 20 {
		wg.Go(func() {
			for range 100 {
				if ok, _ := k.Allow("shared"); ok {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()
	if allowed != 1000 {
		t.Errorf("%d allowed, want exactly 1000", allowed)
	}
}

func TestAttemptWhenTableFull(t *testing.T) {
	k, _ := clocked(2, time.Minute, 2)
	fail(k, "a")
	fail(k, "a")
	fail(k, "b")
	if b, _ := blocked(k, "c"); b {
		t.Fatal("a new key is refused while the table is full")
	}
	fail(k, "c")
	fail(k, "c")
	if b, _ := blocked(k, "c"); b {
		t.Error("failures for an untracked key can't be counted")
	}
	if b, _ := blocked(k, "a"); !b {
		t.Error("keys already tracked stay limited")
	}
}

func TestFailureCountsIfTableFillsDuringAttempt(t *testing.T) {
	k, _ := clocked(1, time.Minute, 1)
	finish, _ := k.Attempt("a")
	// a's bucket is still full, but its attempt is in progress, so pruning
	// keeps it and the table stays full.
	k.Prune(discard, "test")
	fail(k, "b")
	finish(true)
	if b, _ := blocked(k, "a"); !b {
		t.Error("the failure was lost when the table filled during the attempt")
	}
}

// An attacker with far more addresses than the table holds: memory stays
// bounded, a victim already limited stays limited, and new users get in.
func TestFlood(t *testing.T) {
	const maxKeys = 1000
	k, _ := clocked(20, time.Minute, maxKeys)
	for range 20 {
		fail(k, "victim")
	}
	for i := range 50 * maxKeys {
		fail(k, fmt.Sprint("attacker", i))
	}
	if k.Len() > maxKeys {
		t.Errorf("%d keys tracked, want at most %d", k.Len(), maxKeys)
	}
	if b, _ := blocked(k, "victim"); !b {
		t.Error("the flood freed a limited key")
	}
	if b, _ := blocked(k, "newcomer"); b {
		t.Error("the flood locked out a new key")
	}
	if ok, _ := k.Allow("newcomer"); !ok {
		t.Error("the flood locked out a new key")
	}
}

func TestIPKey(t *testing.T) {
	for in, want := range map[string]string{
		"203.0.113.7":          "203.0.113.7",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"2001:db8:1:2::1":      "2001:db8:1:2::/64",
		"::ffff:203.0.113.7":   "::ffff:203.0.113.7",
		"":                     "",
		"not an ip":            "not an ip",
	} {
		if got := IPKey(in); got != want {
			t.Errorf("IPKey(%q) = %q, want %q", in, got, want)
		}
	}
}
