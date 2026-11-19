package clientip

import (
	"net/http"
	"testing"
)

func request(remote string, headers ...string) *http.Request {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Add(headers[i], headers[i+1])
	}
	return r
}

func TestIP(t *testing.T) {
	r, err := NewResolver("127.0.0.1/32, ::1, 10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		req  *http.Request
		want string
	}{
		{"direct, no headers", request("203.0.113.7:5000"), "203.0.113.7"},
		{"direct client can't spoof X-Real-IP", request("203.0.113.7:5000", "X-Real-IP", "1.2.3.4"), "203.0.113.7"},
		{"direct client can't spoof X-Forwarded-For", request("203.0.113.7:5000", "X-Forwarded-For", "1.2.3.4"), "203.0.113.7"},
		{"via nginx, X-Real-IP", request("127.0.0.1:5000", "X-Real-IP", "198.51.100.9"), "198.51.100.9"},
		{"via nginx over IPv6 loopback", request("[::1]:5000", "X-Real-IP", "2001:db8::1"), "2001:db8::1"},
		{"via nginx, no header", request("127.0.0.1:5000"), "127.0.0.1"},
		{"bad X-Real-IP falls back to X-Forwarded-For", request("127.0.0.1:5000", "X-Real-IP", "junk", "X-Forwarded-For", "198.51.100.9"), "198.51.100.9"},
		// The client prepended a fake; nginx appended the real address.
		{"X-Forwarded-For read from the right", request("127.0.0.1:5000", "X-Forwarded-For", "1.2.3.4, 198.51.100.9"), "198.51.100.9"},
		{"trusted hops are skipped", request("127.0.0.1:5000", "X-Forwarded-For", "198.51.100.9, 10.1.2.3"), "198.51.100.9"},
		{"repeated headers are joined", request("127.0.0.1:5000", "X-Forwarded-For", "198.51.100.9", "X-Forwarded-For", "10.1.2.3"), "198.51.100.9"},
		{"garbage stops the walk", request("127.0.0.1:5000", "X-Forwarded-For", "198.51.100.9, nonsense"), "127.0.0.1"},
		{"IPv4-mapped IPv6 is unmapped", request("127.0.0.1:5000", "X-Real-IP", "::ffff:198.51.100.9"), "198.51.100.9"},
		{"no port", request("203.0.113.7"), "203.0.113.7"},
	} {
		if got := r.IP(tc.req); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestZeroResolverTrustsNobody(t *testing.T) {
	var r Resolver
	if got := r.IP(request("127.0.0.1:5000", "X-Real-IP", "1.2.3.4")); got != "127.0.0.1" {
		t.Errorf("got %q", got)
	}
}

func TestNewResolverRejectsGarbage(t *testing.T) {
	if _, err := NewResolver("127.0.0.1/32,not-an-ip"); err == nil {
		t.Error("expected an error")
	}
	if r, err := NewResolver(""); err != nil || len(r.trusted) != 0 {
		t.Errorf("empty list: %v %v", r, err)
	}
}
