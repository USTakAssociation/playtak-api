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

func TestBlockedAndRecord(t *testing.T) {
	k, advance := clocked(3, 15*time.Minute, 100)
	if blocked, _ := k.Blocked("alice"); blocked {
		t.Fatal("unknown key is not blocked")
	}
	for range 3 {
		k.Record("alice")
	}
	blocked, wait := k.Blocked("alice")
	if !blocked || wait != 5*time.Minute {
		t.Fatalf("blocked=%v wait=%v, want blocked for 5m", blocked, wait)
	}
	// Recording past the limit pushes the wait out.
	k.Record("alice")
	if _, wait := k.Blocked("alice"); wait != 10*time.Minute {
		t.Errorf("wait %v, want 10m", wait)
	}
	advance(10 * time.Minute)
	if blocked, _ := k.Blocked("alice"); blocked {
		t.Error("should have refilled")
	}
	k.Record("alice")
	k.Record("alice")
	k.Reset("alice")
	if blocked, _ := k.Blocked("alice"); blocked || k.Len() != 0 {
		t.Error("Reset should forget the key")
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
	k.Record("c") // ignored while full
	if blocked, _ := k.Blocked("c"); !blocked {
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
	k.Record("a")
	k.Record("b")
	// A key that can't be tracked is refused, or a flood of keys would
	// switch the limit off.
	if blocked, wait := k.Blocked("c"); !blocked || wait != 30*time.Second {
		t.Fatalf("full table: blocked=%v wait=%v, want blocked for 30s", blocked, wait)
	}
	if blocked, _ := k.Blocked("a"); blocked {
		t.Error("tracked keys are judged on their own bucket")
	}
	// Once the buckets refill they're pruned, making room.
	advance(time.Minute)
	if blocked, _ := k.Blocked("c"); blocked {
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
