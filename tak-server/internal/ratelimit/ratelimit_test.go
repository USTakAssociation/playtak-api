package ratelimit

import (
	"sync"
	"testing"
	"time"
)

var t0 = time.Unix(1_800_000_000, 0)

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
	k.Prune()
	if k.Len() != 1 {
		t.Errorf("%d keys, want only b", k.Len())
	}
}

func TestMaxKeys(t *testing.T) {
	k, advance := clocked(1, time.Minute, 2)
	k.Allow("a")
	k.Allow("b")
	if ok, wait := k.Allow("c"); ok || wait != time.Minute {
		t.Errorf("a third key while full: ok=%v wait=%v", ok, wait)
	}
	if b, _ := blocked(k, "c"); !b {
		t.Error("an untracked key is blocked while the table is full")
	}
	advance(time.Minute) // a and b refill, so they can be pruned
	if ok, _ := k.Allow("c"); !ok {
		t.Error("a new key should fit once old ones are pruned")
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

func TestBlockedWhenTableFull(t *testing.T) {
	k, advance := clocked(2, time.Minute, 2)
	fail(k, "a")
	fail(k, "b")
	// A key that can't be tracked is refused, or a flood of keys would
	// switch the limit off.
	if b, wait := blocked(k, "c"); !b || wait != 30*time.Second {
		t.Fatalf("full table: blocked=%v wait=%v, want blocked for 30s", b, wait)
	}
	if b, _ := blocked(k, "a"); b {
		t.Error("tracked keys are judged on their own bucket")
	}
	// Once the buckets refill they're pruned, making room.
	advance(time.Minute)
	if b, _ := blocked(k, "c"); b {
		t.Error("refilled buckets should be pruned to make room")
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
