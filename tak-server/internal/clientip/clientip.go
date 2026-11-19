// Package clientip works out the address of the client behind a request,
// trusting headers set by a reverse proxy only when the request actually
// came from that proxy.
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Resolver finds a request's client address. The zero value trusts no proxy
// and always uses the connection's address.
type Resolver struct {
	trusted []netip.Prefix
}

// NewResolver trusts the proxies in cidrs, a comma-separated list such as
// "127.0.0.1/32,::1/128". A bare address is treated as a single host.
func NewResolver(cidrs string) (*Resolver, error) {
	resolver := &Resolver{}
	for _, cidr := range strings.Split(cidrs, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			a, aerr := netip.ParseAddr(cidr)
			if aerr != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", cidr, err)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		resolver.trusted = append(resolver.trusted, p.Masked())
	}
	return resolver, nil
}

// IP returns the client's address for r. If the connection comes from a
// trusted proxy, it's taken from X-Real-IP, or failing that the nearest
// untrusted address in X-Forwarded-For. Otherwise it's the connection's own
// address, whatever the headers say, so clients can't spoof it.
func (r *Resolver) IP(request *http.Request) string {
	peer := Host(request.RemoteAddr)
	if !r.trustedAddr(peer) {
		return peer
	}
	if real := strings.TrimSpace(request.Header.Get("X-Real-IP")); valid(real) {
		return canonical(real)
	}
	// X-Forwarded-For grows to the right as each proxy appends the address
	// it received from; anything left of our own proxies could be forged.
	hops := strings.Split(strings.Join(request.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if !valid(hop) {
			break
		}
		if !r.trustedAddr(hop) {
			return canonical(hop)
		}
	}
	return peer
}

func (r *Resolver) trustedAddr(s string) bool {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range r.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func valid(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}

func canonical(s string) string {
	return netip.MustParseAddr(s).Unmap().String()
}

// Host strips the port from a "host:port" address, returning addr unchanged
// if it has none.
func Host(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}
