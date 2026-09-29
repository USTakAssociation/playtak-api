package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

type fakeSession struct{ version int }

func (fakeSession) Send(protocol.Event)     {}
func (fakeSession) Disconnect()             {}
func (fakeSession) Done() <-chan struct{}   { return nil }
func (s *fakeSession) ProtocolVersion() int { return s.version }

func TestCreateSeek(t *testing.T) {
	srv, _, reg, seeks := newTestServerWithSeeks(t)
	alice := reg.Acquire(players.Record{ID: 1, Name: "alice"})
	oldTab, newTab := &fakeSession{version: 4}, &fakeSession{version: 0}
	reg.Attach(alice, oldTab)
	reg.Attach(alice, newTab)

	body := map[string]any{
		"pntId": 17, "creator": "alice", "opponent": "bobby", "timeContingent": 600, "timeIncrement": 30,
		"komi": 2.0, "boardSize": 6, "pieces": 30, "capstones": 1, "tournament": true, "color": "WHITE",
	}
	auth := []string{"Authorization", "Bearer " + serviceKey}

	if code, _ := post(t, srv, "/api/v1/seeks", body); code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d (the endpoint is PUT)", code)
	}
	if code, _ := put(t, srv, body); code != http.StatusUnauthorized {
		t.Errorf("no key: %d", code)
	}
	if code, _ := put(t, srv, body, "Authorization", "Bearer wrong"); code != http.StatusUnauthorized {
		t.Errorf("wrong key: %d", code)
	}

	code, got := put(t, srv, body, auth...)
	if code != http.StatusOK {
		t.Fatalf("create: %d %v", code, got)
	}
	want := map[string]any{
		"id": 1.0, "pntId": 17.0, "creator": "alice", "opponent": "bobby", "timeContingent": 600.0,
		"timeIncrement": 30.0, "extraTimeTriggerMove": 0.0, "extraTimeAmount": 0.0, "incrementScales": false,
		"opening": "swap", "komi": 2.0, "boardSize": 6.0, "capstones": 1.0, "pieces": 30.0,
		"unrated": false, "tournament": true, "color": "WHITE",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	// Any session can display a swap seek, so it goes to the newest one.
	if s, _ := seeks.Get(1); s.Owner != newTab || s.Komi != 4 || s.PNTID != 17 {
		t.Errorf("seek = %+v", s)
	}

	// A double black stack seek needs protocol 4, which only the older tab has.
	body["opening"] = "Double Black Stack"
	if code, _ := put(t, srv, body, auth...); code != http.StatusOK {
		t.Fatalf("opening seek: %d", code)
	}
	if s, _ := seeks.Get(2); s.Owner != oldTab || s.Opening != protocol.OpeningDoubleBlackStack {
		t.Errorf("seek = %+v", s)
	}

	body["creator"] = "nobody"
	if code, got := put(t, srv, body, auth...); code != http.StatusNotFound || got["error"] != "creator_not_online" {
		t.Errorf("offline creator: %d %v", code, got)
	}
	body["creator"], body["color"] = "alice", "PURPLE"
	if code, _ := put(t, srv, body, auth...); code != http.StatusBadRequest {
		t.Errorf("bad color: %d", code)
	}
}

func TestCreateSeekIncompatible(t *testing.T) {
	srv, _, reg, _ := newTestServerWithSeeks(t)
	p := reg.Acquire(players.Record{ID: 1, Name: "alice"})
	reg.Attach(p, &fakeSession{version: 2})
	code, got := put(t, srv, map[string]any{"creator": "alice", "boardSize": 5, "opening": "double black stack"},
		"Authorization", "Bearer "+serviceKey)
	if code != http.StatusUnprocessableEntity || got["error"] != "incompatible_protocol" {
		t.Errorf("got %d %v", code, got)
	}
}

func TestListSeeks(t *testing.T) {
	srv, _, reg, _ := newTestServerWithSeeks(t)
	p := reg.Acquire(players.Record{ID: 1, Name: "alice"})
	reg.Attach(p, &fakeSession{})
	put(t, srv, map[string]any{"creator": "alice", "boardSize": 5}, "Authorization", "Bearer "+serviceKey)

	resp, err := http.Get(srv.URL + "/api/v1/seeks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []map[string]any
	json.NewDecoder(resp.Body).Decode(&list)
	if resp.StatusCode != http.StatusOK || len(list) != 1 || list[0]["creator"] != "alice" {
		t.Errorf("list: %d %v", resp.StatusCode, list)
	}
	if list[0]["pntId"] != nil || list[0]["opponent"] != nil || list[0]["color"] != "ANY" {
		t.Errorf("unset fields should be null/ANY: %v", list[0])
	}

	resp, _ = http.Get(srv.URL + "/api/v1/seeks/extra")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("only the exact path should match, got %d", resp.StatusCode)
	}
}

func TestServiceEndpointsDisabledWithoutKey(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/seeks", nil)
	req.Header.Set("Authorization", "Bearer ")
	if s.isService(req) {
		t.Error("an empty service key must not authorize anything")
	}
}

func put(t *testing.T, srv *httptest.Server, body any, header ...string) (int, map[string]any) {
	t.Helper()
	return do(t, srv, http.MethodPut, "/api/v1/seeks", body, header...)
}
