package transport

import "testing"

func TestConnLimit(t *testing.T) {
	l, err := NewConnLimit(2, "192.0.2.0/24, 2001:db8:1::5")
	if err != nil {
		t.Fatal(err)
	}
	r1, ok1 := l.Acquire("203.0.113.7")
	_, ok2 := l.Acquire("203.0.113.7")
	if !ok1 || !ok2 {
		t.Fatal("the first two connections should be allowed")
	}
	if _, ok := l.Acquire("203.0.113.7"); ok {
		t.Error("a third connection from the same IP should be refused")
	}
	if _, ok := l.Acquire("203.0.113.8"); !ok {
		t.Error("another IP has its own cap")
	}
	r1()
	r1() // a second release must not free another slot
	if _, ok := l.Acquire("203.0.113.7"); !ok {
		t.Error("a released slot should be reusable")
	}
	if _, ok := l.Acquire("203.0.113.7"); ok {
		t.Error("releasing twice freed two slots")
	}

	// One /64 is one client.
	l.Acquire("2001:db8::1")
	l.Acquire("2001:db8::2")
	if _, ok := l.Acquire("2001:db8::ffff"); ok {
		t.Error("addresses in one /64 should share a cap")
	}

	for _, ip := range []string{"192.0.2.10", "2001:db8:1::5"} {
		for i := range 5 {
			if _, ok := l.Acquire(ip); !ok {
				t.Errorf("exempt %s refused at connection %d", ip, i+1)
			}
		}
	}
}

func TestConnLimitOff(t *testing.T) {
	l, err := NewConnLimit(0, "")
	if err != nil || l != nil {
		t.Fatalf("max 0 should mean no limiter, got %v, %v", l, err)
	}
	for range 3 {
		if release, ok := l.Acquire("203.0.113.7"); !ok {
			t.Fatal("a nil limiter should allow everything")
		} else {
			release()
		}
	}
	if _, err := NewConnLimit(5, "not-an-ip"); err == nil {
		t.Error("expected an error for a bad exempt list")
	}
}
