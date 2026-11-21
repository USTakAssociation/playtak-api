package transport

import (
	"net/netip"
	"sync"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/ratelimit"
)

// ConnLimit caps how many connections one client IP (an IPv6 /64 counts as
// one) may hold open at once, across telnet and WebSocket. Exempt IPs, such
// as a tournament host running many bots, have no cap. A nil *ConnLimit
// allows everything.
type ConnLimit struct {
	max    int
	exempt []netip.Prefix

	mu   sync.Mutex
	open map[string]int
}

// NewConnLimit allows max connections per IP; max <= 0 means no limit, and
// NewConnLimit returns nil. exempt is a comma-separated CIDR list.
func NewConnLimit(max int, exempt string) (*ConnLimit, error) {
	prefixes, err := clientip.ParsePrefixes(exempt)
	if err != nil {
		return nil, err
	}
	if max <= 0 {
		return nil, nil
	}
	return &ConnLimit{max: max, exempt: prefixes, open: make(map[string]int)}, nil
}

// Acquire reports whether ip may open another connection. If it may, call
// release exactly once when the connection closes.
func (l *ConnLimit) Acquire(ip string) (release func(), ok bool) {
	if l == nil || ip == "" || clientip.Contains(l.exempt, ip) {
		return func() {}, true
	}
	key := ratelimit.IPKey(ip)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.open[key] >= l.max {
		return nil, false
	}
	l.open[key]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.open[key]--; l.open[key] <= 0 {
				delete(l.open, key)
			}
		})
	}, true
}
