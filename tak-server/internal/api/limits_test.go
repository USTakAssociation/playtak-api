package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/email"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/player"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/wordfilter"
)

// newLimitedServer has auth limits on. trusted lists proxies whose headers
// are believed; test requests come from 127.0.0.1.
func newLimitedServer(t *testing.T, trusted string) (*httptest.Server, *players.Store) {
	t.Helper()
	conn := dbtest.NewPlayersDB(t)
	store := players.NewStore(conn)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc := auth.NewService(store, players.NewTokenStore(conn), email.Log{Logger: log}, &wordfilter.Filter{}, log)
	authSvc.EnableLimits(true)
	resolver, err := clientip.NewResolver(trusted)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(authSvc, player.NewRegistry(), seek.NewManager(), serviceKey, resolver, log).Handler())
	t.Cleanup(srv.Close)
	return srv, store
}

// status sends a request and returns the status code and Retry-After header.
func status(t *testing.T, srv *httptest.Server, method, path, realIP string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, nil)
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Retry-After")
}

func TestLoginRateLimitedPerUsername(t *testing.T) {
	srv, store := newLimitedServer(t, "")
	store.Create(context.Background(), "alice", "a@example.com", passwordHash)
	for range 10 {
		post(t, srv, "/api/auth/login", map[string]string{"username": "alice", "password": "nope"})
	}
	code, body := post(t, srv, "/api/auth/login", map[string]string{"username": "alice", "password": "password"})
	if code != http.StatusTooManyRequests || body["error"] != "rate_limited" {
		t.Errorf("after 10 failures: %d %v", code, body)
	}
}

func TestRegisterRateLimited(t *testing.T) {
	srv, _ := newLimitedServer(t, "")
	for i := range 5 {
		post(t, srv, "/api/auth/register", map[string]string{"username": fmt.Sprintf("newbie%d", i), "email": fmt.Sprintf("n%d@example.com", i)})
	}
	code, body := post(t, srv, "/api/auth/register", map[string]string{"username": "another1", "email": "z@example.com"})
	if code != http.StatusTooManyRequests || body["error"] != "rate_limited" {
		t.Errorf("6th registration: %d %v", code, body)
	}
}

func TestEndpointLimitsPerIP(t *testing.T) {
	srv, _ := newLimitedServer(t, "")
	for i := range tokenCallsPerIP {
		if code, _ := status(t, srv, "POST", "/api/auth/guest", ""); code != http.StatusOK {
			t.Fatalf("call %d: %d", i+1, code)
		}
	}
	code, retry := status(t, srv, "POST", "/api/auth/guest", "")
	if code != http.StatusTooManyRequests {
		t.Fatalf("past the limit: %d", code)
	}
	if n, err := strconv.Atoi(retry); err != nil || n < 1 || n > 2 {
		t.Errorf("Retry-After %q, want about 2 seconds", retry)
	}
	// refresh and logout share the token-call budget.
	if code, _ := status(t, srv, "POST", "/api/auth/refresh", ""); code != http.StatusTooManyRequests {
		t.Errorf("refresh: %d", code)
	}
	// Seek lists have their own.
	if code, _ := status(t, srv, "GET", "/api/v1/seeks", ""); code != http.StatusOK {
		t.Errorf("seeks: %d", code)
	}
}

func TestSeekListLimit(t *testing.T) {
	srv, _ := newLimitedServer(t, "")
	for range seekListsPerIP {
		status(t, srv, "GET", "/api/v1/seeks", "")
	}
	if code, _ := status(t, srv, "GET", "/api/v1/seeks", ""); code != http.StatusTooManyRequests {
		t.Errorf("past the limit: %d", code)
	}
}

func TestTrustedProxyHeader(t *testing.T) {
	// Through a trusted proxy, each X-Real-IP is its own client.
	srv, _ := newLimitedServer(t, "127.0.0.1/32")
	for i := range tokenCallsPerIP + 5 {
		if code, _ := status(t, srv, "POST", "/api/auth/guest", fmt.Sprintf("198.51.100.%d", i)); code != http.StatusOK {
			t.Fatalf("client %d limited: %d", i, code)
		}
	}
	for range tokenCallsPerIP {
		status(t, srv, "POST", "/api/auth/guest", "203.0.113.9")
	}
	if code, _ := status(t, srv, "POST", "/api/auth/guest", "203.0.113.9"); code != http.StatusTooManyRequests {
		t.Errorf("one client past its limit: %d", code)
	}
}

func TestUntrustedHeaderIgnored(t *testing.T) {
	// Without trusting the proxy, the header can't be used to dodge limits.
	srv, _ := newLimitedServer(t, "")
	for i := range tokenCallsPerIP {
		status(t, srv, "POST", "/api/auth/guest", fmt.Sprintf("198.51.100.%d", i))
	}
	if code, _ := status(t, srv, "POST", "/api/auth/guest", "198.51.100.250"); code != http.StatusTooManyRequests {
		t.Errorf("spoofed header bypassed the limit: %d", code)
	}
}
