package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/ratelimit"
)

func limitedFixture(t *testing.T, byUsername bool) fixture {
	t.Helper()
	f := newFixture(t)
	f.svc.EnableLimits(byUsername)
	return f
}

// ip returns a distinct address per attempt, so per-IP limits don't interfere.
func ip(i int) string { return fmt.Sprintf("198.51.100.%d", i) }

func TestFailedLoginsPerUsernameAndIP(t *testing.T) {
	f := limitedFixture(t, true)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	f.addPlayer(t, "bobby", "b@example.com", "secret123")
	const attacker = "203.0.113.66"
	for i := range failuresPerNameIP {
		if _, err := f.svc.Login(ctx, "alice", "wrong", attacker); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	// From that IP the name is now locked, even with the right password and
	// in another case.
	for _, name := range []string{"alice", "ALICE"} {
		_, err := f.svc.Login(ctx, name, "secret123", attacker)
		var rl *RateLimitError
		if !errors.As(err, &rl) || rl.RetryAfter <= 0 || rl.RetryAfter > 2*time.Minute {
			t.Errorf("%s after %d failures: %v", name, failuresPerNameIP, err)
		}
	}
	// The attacker can't lock alice out from anywhere else.
	if _, err := f.svc.Login(ctx, "alice", "secret123", "198.51.100.1"); err != nil {
		t.Errorf("alice locked out by another IP's failures: %v", err)
	}
	if _, err := f.svc.Login(ctx, "bobby", "secret123", attacker); err != nil {
		t.Errorf("other names from that IP are unaffected: %v", err)
	}
}

func TestFailedLoginsPerUsernameAcrossIPs(t *testing.T) {
	f := limitedFixture(t, true)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	f.addPlayer(t, "bobby", "b@example.com", "secret123")
	// One guess per IP stays under every per-IP limit.
	for i := range failuresPerName {
		if _, err := f.svc.Login(ctx, "alice", "wrong", ip(i)); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	_, err := f.svc.Login(ctx, "Alice", "secret123", ip(200))
	var rl *RateLimitError
	if !errors.As(err, &rl) || rl.RetryAfter <= 0 || rl.RetryAfter > time.Minute {
		t.Errorf("after %d failures across IPs: %v", failuresPerName, err)
	}
	if _, err := f.svc.Login(ctx, "bobby", "secret123", ip(201)); err != nil {
		t.Errorf("other names are unaffected: %v", err)
	}
	// Unknown names are counted too, so they can't be probed endlessly.
	for range failuresPerNameIP {
		f.svc.Login(ctx, "ghost", "x", ip(202))
	}
	if _, err := f.svc.Login(ctx, "ghost", "x", ip(202)); !errors.Is(err, ErrRateLimited) {
		t.Errorf("unknown name: %v", err)
	}
}

func TestSuccessfulLoginResetsFailures(t *testing.T) {
	f := limitedFixture(t, true)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	login := func(password, ip string) error {
		_, err := f.svc.Login(ctx, "alice", password, ip)
		return err
	}
	// Per name and IP.
	for range failuresPerNameIP - 1 {
		login("wrong", "203.0.113.7")
	}
	if err := login("secret123", "203.0.113.7"); err != nil {
		t.Fatal(err)
	}
	for range failuresPerNameIP - 1 {
		login("wrong", "203.0.113.7")
	}
	if err := login("secret123", "203.0.113.7"); err != nil {
		t.Errorf("a success should have cleared this IP's earlier failures: %v", err)
	}
	// Per name across IPs.
	for i := range failuresPerName - 1 {
		login("wrong", ip(i))
	}
	if err := login("secret123", ip(150)); err != nil {
		t.Fatal(err)
	}
	for i := range failuresPerName - 1 {
		login("wrong", ip(i))
	}
	if err := login("secret123", ip(151)); err != nil {
		t.Errorf("a success should have cleared the earlier failures: %v", err)
	}
}

func TestPerIPOnlyMode(t *testing.T) {
	f := limitedFixture(t, false)
	ctx := context.Background()
	// Under the per-IP limit, but over the per-name-and-IP one.
	for range failuresPerNameIP + 5 {
		f.svc.Login(ctx, "ghost", "wrong", "203.0.113.7")
	}
	if _, err := f.svc.Login(ctx, "ghost", "wrong", "203.0.113.7"); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("names aren't limited in per-IP mode: %v", err)
	}
}

func TestFailedLoginsPerIP(t *testing.T) {
	f := limitedFixture(t, false)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	// Many players behind one IP all logging in successfully, e.g. after a
	// restart, are never refused.
	for i := range failuresPerIP + 5 {
		if _, err := f.svc.Login(ctx, "alice", "secret123", "203.0.113.7"); err != nil {
			t.Fatalf("successful login %d refused: %v", i+1, err)
		}
	}
	for i := range failuresPerIP {
		if _, err := f.svc.Login(ctx, fmt.Sprintf("user%d", i), "x", "203.0.113.7"); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	// After 20 failures the IP is limited across all names, even for a
	// correct password, and a success doesn't clear it.
	if _, err := f.svc.Login(ctx, "alice", "secret123", "203.0.113.7"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("past the per-IP limit: %v", err)
	}
	if _, err := f.svc.Login(ctx, "alice", "secret123", "203.0.113.8"); err != nil {
		t.Errorf("other IPs are unaffected: %v", err)
	}
	// An unknown address isn't limited (rather than lumping everyone together).
	for range failuresPerIP + 1 {
		if _, err := f.svc.Login(ctx, "nobody", "x", ""); errors.Is(err, ErrRateLimited) {
			t.Fatal("empty IP must not be limited")
		}
	}
}

// gatedStore holds every ByName call until n have started, so the logins
// are sure to overlap.
type gatedStore struct {
	PlayerStore
	n       int32
	started atomic.Int32
	open    chan struct{}
}

func (g *gatedStore) ByName(ctx context.Context, name string) (players.Record, error) {
	if g.started.Add(1) == g.n {
		close(g.open)
	}
	select {
	case <-g.open:
	case <-time.After(5 * time.Second): // fewer got in than the limit
	}
	return g.PlayerStore.ByName(ctx, name)
}

func TestParallelLoginsCantPassTheLimit(t *testing.T) {
	f := limitedFixture(t, false)
	ctx := context.Background()
	gate := &gatedStore{PlayerStore: f.store, n: failuresPerIP, open: make(chan struct{})}
	f.svc.players = gate
	var wg sync.WaitGroup
	for range 3 * failuresPerIP {
		wg.Go(func() { f.svc.Login(ctx, "ghost", "x", "203.0.113.7") })
	}
	wg.Wait()
	// Once failuresPerIP are being checked, the rest must be refused
	// without reaching the store, though none has failed yet.
	if n := gate.started.Load(); n != failuresPerIP {
		t.Errorf("%d logins checked in parallel, want %d", n, failuresPerIP)
	}
}

type panickingStore struct{ PlayerStore }

func (panickingStore) ByName(context.Context, string) (players.Record, error) {
	panic("store blew up")
}

// net/http recovers a panicking handler, so a login that panics mustn't
// leave its attempt counted.
func TestLoginPanicFreesItsAttempt(t *testing.T) {
	f := limitedFixture(t, false)
	ctx := context.Background()
	f.svc.players = panickingStore{f.store}
	func() {
		defer func() { recover() }()
		f.svc.Login(ctx, "ghost", "x", "203.0.113.7")
	}()
	f.svc.players = f.store
	for i := range failuresPerIP {
		if _, err := f.svc.Login(ctx, "ghost", "x", "203.0.113.7"); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("attempt %d after the panic: %v", i+1, err)
		}
	}
}

// An attacker floods the login limits with made-up names from many IPs,
// far more than the tables hold.
func TestLoginFlood(t *testing.T) {
	f := limitedFixture(t, true)
	const maxKeys = 50
	f.svc.limits.failuresByIP = ratelimit.New(failuresPerIP, time.Minute, maxKeys)
	f.svc.limits.failuresByName = ratelimit.New(failuresPerName, 15*time.Minute, maxKeys)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	f.addPlayer(t, "bobby", "b@example.com", "secret123")
	// alice was being guessed before the flood, so her name is limited.
	for i := range failuresPerName {
		f.svc.Login(ctx, "alice", "wrong", fmt.Sprintf("192.0.2.%d", i))
	}
	for i := range 10 * maxKeys {
		ip := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
		if _, err := f.svc.Login(ctx, fmt.Sprint("ghost", i), "x", ip); !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("flood attempt %d: %v", i, err)
		}
	}
	if n := f.svc.limits.failuresByName.Len(); n > maxKeys {
		t.Errorf("%d names tracked, want at most %d", n, maxKeys)
	}
	// New users still get in: a full table doesn't lock them out.
	if _, err := f.svc.Login(ctx, "bobby", "secret123", "198.51.100.1"); err != nil {
		t.Errorf("bobby locked out by the flood: %v", err)
	}
	// alice's name was tracked before the flood, so it stays limited.
	if _, err := f.svc.Login(ctx, "alice", "secret123", "198.51.100.2"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("the flood lifted alice's limit: %v", err)
	}
	// The cost: while the tables are full, guesses for a name they don't hold
	// go uncounted (the attacker's own IPs can still be limited if tracked).
	for i := range failuresPerName + 1 {
		if _, err := f.svc.Login(ctx, "carol", "wrong", fmt.Sprintf("198.51.101.%d", i)); errors.Is(err, ErrRateLimited) {
			t.Fatal("carol's name isn't tracked, so it can't be limited while full")
		}
	}
}

func TestFailedLoginsPerIPv6Network(t *testing.T) {
	f := limitedFixture(t, false)
	ctx := context.Background()
	// Rotating addresses within one /64 shares one limit.
	for i := range failuresPerIP {
		f.svc.Login(ctx, "ghost", "x", fmt.Sprintf("2001:db8:1:2::%x", i+1))
	}
	if _, err := f.svc.Login(ctx, "ghost", "x", "2001:db8:1:2:ffff::1"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("same /64: %v", err)
	}
	if _, err := f.svc.Login(ctx, "ghost", "x", "2001:db8:1:3::1"); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("another /64 is unaffected: %v", err)
	}
}

func TestRegistrationsPerIP(t *testing.T) {
	f := limitedFixture(t, true)
	ctx := context.Background()
	for i := range registrationsPerIP {
		// Invalid attempts count as well.
		name := fmt.Sprintf("newbie%d", i)
		if i == 0 {
			name = "x"
		}
		f.svc.Register(ctx, name, fmt.Sprintf("n%d@example.com", i), "203.0.113.7")
	}
	if _, err := f.svc.Register(ctx, "another1", "z@example.com", "203.0.113.7"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("past the limit: %v", err)
	}
	if _, err := f.svc.Register(ctx, "another2", "y@example.com", "203.0.113.8"); err != nil {
		t.Errorf("other IPs are unaffected: %v", err)
	}
}

func TestResetEmailsPerName(t *testing.T) {
	f := limitedFixture(t, true)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	for range resetsPerName {
		if err := f.svc.SendResetToken(ctx, "alice", "a@example.com"); err != nil {
			t.Fatal(err)
		}
		f.mail.next(t)
	}
	if err := f.svc.SendResetToken(ctx, "alice", "a@example.com"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("past the limit: %v", err)
	}
	select {
	case m := <-f.mail:
		t.Errorf("a limited request sent mail: %+v", m)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestResetEmailsCountOnlyWhenSent(t *testing.T) {
	f := limitedFixture(t, true)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	for i := range 10 {
		f.svc.SendResetToken(ctx, "alice", "wrong@example.com")
		f.svc.SendResetToken(ctx, fmt.Sprintf("madeup%d", i), "a@example.com")
	}
	if n := f.svc.limits.resetByName.Len(); n != 0 {
		t.Errorf("failed requests added %d keys", n)
	}
	if err := f.svc.SendResetToken(ctx, "alice", "a@example.com"); err != nil {
		t.Errorf("owner's reset after wrong-email requests: %v", err)
	}
	f.mail.next(t)
}

func TestNoLimitsUnlessEnabled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	for range failuresPerName + failuresPerIP {
		f.svc.Login(ctx, "ghost", "wrong", "203.0.113.7")
	}
	if _, err := f.svc.Login(ctx, "alice", "secret123", "203.0.113.7"); err != nil {
		t.Errorf("limits must be off by default: %v", err)
	}
	f.svc.PruneLimits() // harmless when off
}

func TestPruneLimits(t *testing.T) {
	f := limitedFixture(t, true)
	f.svc.Login(context.Background(), "nobody", "x", "203.0.113.7")
	if f.svc.limits.failuresByIP.Len() != 1 || f.svc.limits.failuresByName.Len() != 1 {
		t.Fatal("expected tracked keys")
	}
	f.svc.PruneLimits() // nothing has refilled yet
	if f.svc.limits.failuresByIP.Len() != 1 {
		t.Error("an unrefilled key must be kept")
	}
}

func TestRateLimitError(t *testing.T) {
	err := error(&RateLimitError{RetryAfter: 90 * time.Second})
	if !errors.Is(err, ErrRateLimited) || err.Error() != "too many attempts, retry in 1m30s" {
		t.Errorf("%v", err)
	}
}

func TestSuccessDoesntClearIPFailures(t *testing.T) {
	f := limitedFixture(t, false)
	ctx := context.Background()
	f.addPlayer(t, "alice", "a@example.com", "secret123")
	for i := range failuresPerIP - 1 {
		f.svc.Login(ctx, fmt.Sprintf("guess%d", i), "x", "203.0.113.7")
	}
	// Someone else on the same IP logs in; the guesses still count.
	if _, err := f.svc.Login(ctx, "alice", "secret123", "203.0.113.7"); err != nil {
		t.Fatal(err)
	}
	f.svc.Login(ctx, "guess99", "x", "203.0.113.7")
	if _, err := f.svc.Login(ctx, "guess100", "x", "203.0.113.7"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("the 20th failure should limit the IP despite the success in between: %v", err)
	}
}
