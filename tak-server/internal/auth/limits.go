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
	failuresPerIP      = 20 // failed logins per minute
	failuresPerName    = 10 // failed logins per 15 minutes
	registrationsPerIP = 5  // per hour
	resetsPerName      = 3  // reset emails per hour
	maxTrackedKeys     = 100_000
)

type limiters struct {
	failuresByIP   *ratelimit.Keyed
	failuresByName *ratelimit.Keyed // nil when only IPs are limited
	registerByIP   *ratelimit.Keyed
	resetByName    *ratelimit.Keyed
}

// EnableLimits turns on rate limiting. With byUsername, failed logins are
// also counted per username, which stops password guessing spread over many
// IPs, at the cost of letting someone lock a name out for a while by
// failing on purpose. Without it, only per-IP limits apply.
func (s *Service) EnableLimits(byUsername bool) {
	l := &limiters{
		failuresByIP: ratelimit.New(failuresPerIP, time.Minute, maxTrackedKeys),
		registerByIP: ratelimit.New(registrationsPerIP, time.Hour, maxTrackedKeys),
		resetByName:  ratelimit.New(resetsPerName, time.Hour, maxTrackedKeys),
	}
	if byUsername {
		l.failuresByName = ratelimit.New(failuresPerName, 15*time.Minute, maxTrackedKeys)
	}
	s.limits = l
}

// PruneLimits forgets keys whose limits have reset, to keep memory small.
func (s *Service) PruneLimits() {
	if s.limits == nil {
		return
	}
	for _, k := range []*ratelimit.Keyed{s.limits.failuresByIP, s.limits.failuresByName, s.limits.registerByIP, s.limits.resetByName} {
		if k != nil {
			k.Prune()
		}
	}
}

// nameKey makes case variants of a name share one limit.
func nameKey(name string) string { return strings.ToLower(name) }

// blocked returns a *RateLimitError, and logs it, if key has no failures
// left in k. An empty key (an unknown IP) is never blocked.
func (s *Service) blocked(k *ratelimit.Keyed, what, key, ip string) error {
	if k == nil || key == "" {
		return nil
	}
	if blocked, wait := k.Blocked(key); blocked {
		s.log.Warn("rate limited", "limit", what, "key", key, "ip", ip, "retry_in", wait.Round(time.Second))
		return &RateLimitError{RetryAfter: wait}
	}
	return nil
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
