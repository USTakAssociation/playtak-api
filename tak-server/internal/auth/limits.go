package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/ratelimit"
)

// ErrRateLimited is matched by every *RateLimitError.
var ErrRateLimited = errors.New("too many attempts")

// RateLimitError means a limit was reached; RetryAfter says when to try again.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("too many attempts, retry in %v", e.RetryAfter.Round(time.Second))
}

func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// Limits on credential checks and on actions that send email. They apply
// to every entry point: the text protocol over telnet and WebSocket, and REST.
const (
	failuresPerIP      = 20  // failed logins per minute
	failuresPerNameIP  = 10  // failed logins per name from one IP, per 15 minutes
	failuresPerName    = 100 // failed logins per name from all IPs, per hour
	registrationsPerIP = 5   // per hour
	resetsPerName      = 3   // reset emails per hour
	maxTrackedKeys     = 100_000
)

// limiters holds the rate limits. A nil limiter means that limit is off; the
// zero value, before EnableLimits, has every limit off.
type limiters struct {
	failuresByIP     *ratelimit.Keyed
	failuresByNameIP *ratelimit.Keyed // nil when only IPs are limited
	failuresByName   *ratelimit.Keyed // nil when only IPs are limited
	registerByIP     *ratelimit.Keyed
	resetByName      *ratelimit.Keyed
}

// EnableLimits turns on rate limiting. With byUsername, failed logins are
// also counted per username: strictly per username and IP, so one IP can't
// lock a name out for everyone else, and loosely per username across all
// IPs, which caps guessing spread over many IPs. Without it, only per-IP
// limits apply.
func (s *Service) EnableLimits(byUsername bool) {
	l := limiters{
		failuresByIP: ratelimit.New(failuresPerIP, time.Minute, maxTrackedKeys),
		registerByIP: ratelimit.New(registrationsPerIP, time.Hour, maxTrackedKeys),
		resetByName:  ratelimit.New(resetsPerName, time.Hour, maxTrackedKeys),
	}
	if byUsername {
		l.failuresByNameIP = ratelimit.New(failuresPerNameIP, 15*time.Minute, maxTrackedKeys)
		l.failuresByName = ratelimit.New(failuresPerName, time.Hour, maxTrackedKeys)
	}
	s.limits = l
}

// PruneLimits forgets keys whose limits have reset, to keep memory small,
// and warns about any limit whose table filled up.
func (s *Service) PruneLimits() {
	for what, k := range map[string]*ratelimit.Keyed{
		"failed logins per IP":          s.limits.failuresByIP,
		"failed logins per name and IP": s.limits.failuresByNameIP,
		"failed logins per name":        s.limits.failuresByName,
		"registrations per IP":          s.limits.registerByIP,
		"reset emails per name":         s.limits.resetByName,
	} {
		if k != nil {
			k.Prune(s.log, what)
		}
	}
}

// nameKey makes case variants of a name share one limit.
func nameKey(name string) string { return strings.ToLower(name) }

// nameIPKey keys a name's failures from one IP; "" (not limited) for an
// unknown IP.
func nameIPKey(name, ip string) string {
	if k := ratelimit.IPKey(ip); k != "" {
		return nameKey(name) + " " + k
	}
	return ""
}

// startLogin starts counting a login against ip, name and ip, and name,
// returning a *RateLimitError if any has no failures left. Otherwise call
// finish exactly once, saying whether the login failed.
func (s *Service) startLogin(name, ip string) (finish func(failed bool), err error) {
	checks := []struct {
		k         *ratelimit.Keyed
		what, key string
	}{
		{s.limits.failuresByIP, "failed logins per IP", ratelimit.IPKey(ip)},
		{s.limits.failuresByNameIP, "failed logins per name and IP", nameIPKey(name, ip)},
		{s.limits.failuresByName, "failed logins per name", nameKey(name)},
	}
	var finishes []func(bool)
	finishAll := func(failed bool) {
		for _, f := range finishes {
			f(failed)
		}
	}
	for _, c := range checks {
		f, err := s.attempt(c.k, c.what, c.key, ip)
		if err != nil {
			finishAll(false)
			return nil, err
		}
		finishes = append(finishes, f)
	}
	return finishAll, nil
}

// attempt starts an attempt for key in k, logging and returning a
// *RateLimitError if it has no failures left. An empty key (an unknown IP)
// is never limited.
func (s *Service) attempt(k *ratelimit.Keyed, what, key, ip string) (func(failed bool), error) {
	if k == nil || key == "" {
		return func(bool) {}, nil
	}
	finish, wait := k.Attempt(key)
	if finish == nil {
		s.log.Warn("rate limited", "limit", what, "key", key, "ip", ip, "retry_in", wait.Round(time.Second))
		return nil, &RateLimitError{RetryAfter: wait}
	}
	return finish, nil
}

// allow takes an event from k for key, logging and returning a
// *RateLimitError if there's none. An empty key (an unknown IP) isn't limited.
func (s *Service) allow(k *ratelimit.Keyed, what, key string) error {
	if k == nil || key == "" {
		return nil
	}
	if ok, wait := k.Allow(key); !ok {
		s.log.Warn("rate limited", "limit", what, "key", key, "retry_in", wait.Round(time.Second))
		return &RateLimitError{RetryAfter: wait}
	}
	return nil
}
