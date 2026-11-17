package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/email"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
)

// bcrypt hash of "password"
const passwordHash = "$2a$10$JpGWa00gDsDtJK0Ttq/dD.F2zTj9kCkdcHwO5pIZFfWbT3CfDY/C6"

const serviceKey = "test-service-key"

func newTestServer(t *testing.T) (*httptest.Server, *players.Store, *online.Registry) {
	srv, store, reg, _ := newTestServerWithSeeks(t)
	return srv, store, reg
}

func newTestServerWithSeeks(t *testing.T) (*httptest.Server, *players.Store, *online.Registry, *seek.Manager) {
	t.Helper()
	conn := dbtest.NewPlayersDB(t)
	store := players.NewStore(conn)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc := auth.NewService(store, players.NewTokenStore(conn), email.Log{Logger: log}, log)
	reg := online.NewRegistry()
	seeks := seek.NewManager()
	srv := httptest.NewServer(NewServer(authSvc, reg, seeks, serviceKey, &clientip.Resolver{}, PublicData{}, log).Handler())
	t.Cleanup(srv.Close)
	return srv, store, reg, seeks
}

func post(t *testing.T, srv *httptest.Server, path string, body any, header ...string) (int, map[string]any) {
	t.Helper()
	return do(t, srv, http.MethodPost, path, body, header...)
}

// do sends body as JSON, with an optional header name and value, and decodes
// a JSON object response.
func do(t *testing.T, srv *httptest.Server, method, path string, body any, header ...string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, srv.URL+path, &buf)
	if len(header) == 2 {
		req.Header.Set(header[0], header[1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestLoginRefreshLogout(t *testing.T) {
	srv, store, _ := newTestServer(t)
	store.Create(context.Background(), "alice", "a@example.com", passwordHash)

	if code, body := post(t, srv, "/api/auth/login", map[string]string{"username": "alice", "password": "nope"}); code != http.StatusUnauthorized || body["error"] != "invalid_credentials" {
		t.Errorf("bad password: %d %v", code, body)
	}

	code, body := post(t, srv, "/api/auth/login", map[string]string{"username": "alice", "password": "password"})
	if code != http.StatusOK || body["name"] != "alice" || body["accessToken"] == "" || body["refreshToken"] == "" {
		t.Fatalf("login: %d %v", code, body)
	}
	refresh := body["refreshToken"].(string)

	code, body = post(t, srv, "/api/auth/refresh", map[string]string{"refreshToken": refresh})
	if code != http.StatusOK || body["name"] != "alice" || body["refreshToken"] == refresh {
		t.Fatalf("refresh: %d %v", code, body)
	}
	if code, _ := post(t, srv, "/api/auth/refresh", map[string]string{"refreshToken": refresh}); code != http.StatusUnauthorized {
		t.Errorf("reused refresh token: %d", code)
	}

	access, refresh2 := body["accessToken"].(string), body["refreshToken"].(string)
	if code, _ := post(t, srv, "/api/auth/logout", map[string]string{"refreshToken": refresh2}, "Authorization", "Bearer "+access); code != http.StatusNoContent {
		t.Errorf("logout: %d", code)
	}
	if code, _ := post(t, srv, "/api/auth/refresh", map[string]string{"refreshToken": refresh2}); code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: %d", code)
	}
}

func TestRegister(t *testing.T) {
	srv, store, _ := newTestServer(t)
	store.Create(context.Background(), "alice", "", passwordHash)

	for _, tc := range []struct {
		username, email string
		code            int
		err             string
	}{
		{"newbie", "n@example.com", http.StatusCreated, ""},
		{"a1ice", "x@example.com", http.StatusConflict, "name_taken"},
		{"ab", "x@example.com", http.StatusBadRequest, "invalid_username"},
		{"validname", "nope", http.StatusBadRequest, "invalid_email"},
		{"TheGuest", "x@example.com", http.StatusBadRequest, "guest_name"},
	} {
		code, body := post(t, srv, "/api/auth/register", map[string]string{"username": tc.username, "email": tc.email})
		if code != tc.code || (tc.err != "" && body["error"] != tc.err) {
			t.Errorf("register %q: %d %v, want %d %q", tc.username, code, body, tc.code, tc.err)
		}
	}
}

func TestGuest(t *testing.T) {
	srv, _, reg := newTestServer(t)
	code, body := post(t, srv, "/api/auth/guest", nil)
	if code != http.StatusOK {
		t.Fatalf("guest: %d", code)
	}
	token, _ := body["guestToken"].(string)
	if !regexp.MustCompile(`^[a-z]{20}$`).MatchString(token) {
		t.Errorf("guestToken %q is not usable with Login Guest", token)
	}
	if p := reg.Guest(token); p.Name != body["name"] {
		t.Errorf("token resolves to %q, want %q", p.Name, body["name"])
	}
}

func TestInvalidJSON(t *testing.T) {
	srv, _, _ := newTestServer(t)
	resp, err := http.Post(srv.URL+"/api/auth/login", "application/json", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status %d", resp.StatusCode)
	}
}
