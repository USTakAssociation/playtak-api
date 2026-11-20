package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
)

const historyGames = `
INSERT INTO games (id, date, size, player_white, player_black, notation, result, timertime, timerinc, rating_white, rating_black,
	unrated, tournament, komi, pieces, capstones, rating_change_white, rating_change_black, extra_time_amount, extra_time_trigger, increment_scales, opening) VALUES
(1, 1400000000000, 5, 'alice', 'bob', 'P A1,P E5', 'R-0', 600, 10, 1500, 1400, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 'swap'),
(2, 1500000000000, 6, 'bob', 'alice', 'P A1,P F6 C,M A1 A2 1,P C3 W,M A2 C2 1 1', '0-R', 900, 0, 0, 1600, 0, 1, 4, 30, 1, 0, 0, 600, 35, 1, 'double black stack'),
(3, 1600000000000, 6, 'Alice', 'carol', '', '1/2-1/2', 600, 10, 1000, 1000, 1, 0, 0, -1, -1, 0, 0, 0, 0, 0, 'swap'),
(4, 1700000000000, 5, 'carol', 'bob', '', '1-0', 180, 5, 1000, 1000, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 'swap'),
(5, 1800000000000, 5, 'al%', 'dave', '', '0-F', 180, 5, 1000, 1000, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 'swap'),
(6, 1900000000000, 5, 'alice', 'alice', '', '1/2-1/2', 600, 10, 1000, 1000, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 'swap');`

const historyPlayers = `
INSERT INTO players (id, name, rating, ratedgames, maxrating, participation_rating, isbot, unrated, ratingbase) VALUES
(1, 'alice', 1612.5, 40, 1700, 1600, 0, 0, 0),
(2, 'bob', 1400.25, 10, 1450, 1400, 1, 0, 0),
(3, 'carol', 1300, 0, 1300, 1300, 0, 0, 0),
(4, 'dave', 1800, 99, 1800, 1750, 0, 1, 0),
(5, 'Alice', 1100, 5, 1100, 1100, 0, 0, 0);`

func newPublicServer(t *testing.T, eventsURL, publicDB string) *httptest.Server {
	t.Helper()
	gamesDB, playersDB := dbtest.NewGamesDB(t), dbtest.NewPlayersDB(t)
	if _, err := gamesDB.Exec(historyGames); err != nil {
		t.Fatal(err)
	}
	if _, err := playersDB.Exec(historyPlayers); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	public := PublicData{Games: games.NewStore(gamesDB), Players: players.NewStore(playersDB), EventsCSVURL: eventsURL, PublicGamesDB: publicDB}
	srv := httptest.NewServer(NewServer(nil, nil, nil, "", &clientip.Resolver{}, public, log).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, srv *httptest.Server, path string, v any) int {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" && strings.HasPrefix(path, "/v1/") {
		t.Errorf("%s: no CORS header", path)
	}
	if v != nil {
		json.NewDecoder(resp.Body).Decode(v)
	}
	return resp.StatusCode
}

func TestGamesHistoryFilters(t *testing.T) {
	srv := newPublicServer(t, "", "")
	for query, want := range map[string][]int{
		"":                                       {6, 5, 4, 3, 2, 1},
		"?order=asc":                             {1, 2, 3, 4, 5, 6},
		"?player_white=alice":                    {6, 3, 1},    // case-insensitive
		"?player_white=al%25":                    {5},          // % is a literal character, not a wildcard
		"?player_white=alice&mirror=true":        {6, 3, 2, 1}, // the self-game once
		"?player_black=alice&mirror=true&size=5": {6, 1},
		"?player_black=bob&game_result=X-0":      {4, 1},
		"?player_white=bob&game_result=0-X&mirror=true": {4, 2, 1}, // every game bob lost, as either color
		"?game_result=0-X":                                 {5, 2},
		"?game_result=1/2-1/2&mirror=true":                 {6, 3},
		"?player_white=alice&player_black=bob&mirror=true": {2, 1},
		"?id=2-4":                            {4, 3, 2},
		"?id=4-2":                            {4, 3, 2},
		"?id=1,5":                            {5, 1},
		"?id=3":                              {3},
		"?date=>1600000000000":               {6, 5, 4},
		"?date=<1600000000000":               {2, 1},
		"?size=6":                            {3, 2},
		"?komi=4":                            {2},
		"?type=tournament":                   {2},
		"?type=unrated":                      {3},
		"?type=normal":                       {6, 5, 4, 1},
		"?sort=date&order=ASC&timertime=180": {4, 5},
	} {
		var body page[games.Game]
		if code := getJSON(t, srv, "/v1/games-history"+query, &body); code != http.StatusOK {
			t.Errorf("%q: status %d", query, code)
			continue
		}
		var ids []int
		for _, g := range body.Items {
			ids = append(ids, g.ID)
		}
		if !slices.Equal(ids, want) || body.Total != len(want) {
			t.Errorf("%q: ids %v total %d, want %v", query, ids, body.Total, want)
		}
	}
}

func TestGamesHistoryPagingAndErrors(t *testing.T) {
	srv := newPublicServer(t, "", "")
	var body page[games.Game]
	getJSON(t, srv, "/v1/games-history?limit=2&page=1", &body)
	if len(body.Items) != 2 || body.Items[0].ID != 4 || body.Total != 6 || body.Page != 2 || body.PerPage != 2 || body.TotalPages != 3 {
		t.Errorf("page 1 = %+v", body)
	}
	getJSON(t, srv, "/v1/games-history?limit=2&skip=3", &body)
	if len(body.Items) != 2 || body.Items[0].ID != 3 || body.Page != 1 {
		t.Errorf("skip 3 = %+v", body)
	}
	getJSON(t, srv, "/v1/games-history?player_black=alice&mirror=true&order=ASC&limit=2&page=1", &body)
	if len(body.Items) != 2 || body.Items[0].ID != 3 || body.Items[1].ID != 6 || body.Total != 4 {
		t.Errorf("mirrored page 1 = %+v", body)
	}
	getJSON(t, srv, "/v1/games-history?limit=999999", &body)
	if body.PerPage != maxPageSize {
		t.Errorf("perPage = %d, want the cap %d", body.PerPage, maxPageSize)
	}
	// A page so far out that limit*page would overflow is just empty.
	if code := getJSON(t, srv, "/v1/games-history?limit=1000&page=9223372036854775807", &body); code != http.StatusOK || len(body.Items) != 0 {
		t.Errorf("huge page: %d %+v", code, body)
	}
	ids := strings.Repeat("1,", maxFilterValues-1) + "2"
	if code := getJSON(t, srv, "/v1/games-history?id="+ids, &body); code != http.StatusOK || body.Total != 2 {
		t.Errorf("%d ids: %d, total %d", maxFilterValues, code, body.Total)
	}
	for _, query := range []string{"?sort=password", "?sort=id%3BDROP%20TABLE%20games", "?order=sideways", "?id=abc", "?date=>x", "?size=big", "?type=secret", "?id=" + ids + ",3"} {
		if code := getJSON(t, srv, "/v1/games-history"+query, nil); code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", query, code)
		}
	}
}

func TestGameByIDAndPTN(t *testing.T) {
	srv := newPublicServer(t, "", "")
	var g games.Game
	if code := getJSON(t, srv, "/v1/games-history/2", &g); code != http.StatusOK || g.PlayerWhite != "bob" || g.Opening != "double black stack" {
		t.Errorf("game 2: %d %+v", code, g)
	}
	if code := getJSON(t, srv, "/v1/games-history/99", nil); code != http.StatusNotFound {
		t.Errorf("missing game: %d", code)
	}
	if code := getJSON(t, srv, "/v1/games-history/abc", nil); code != http.StatusBadRequest {
		t.Errorf("bad id: %d", code)
	}

	resp, err := http.Get(srv.URL + "/v1/games-history/ptn/2")
	if err != nil {
		t.Fatal(err)
	}
	ptn, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	want := `[Site "PlayTak.com"]
[Event "Online Play"]
[Date "2017.07.14"]
[Time "02:40:00"]
[Player1 "bob"]
[Player2 "alice"]
[Rating2 "1600"]
[Clock "15:0 @35 +10:0"]
[Result "0-R"]
[Size "6"]
[Komi "2"]
[Flats "30"]
[Caps "1"]
[Opening "double black stack"]


1. 2a1 Cf6
2. 1a1+1 Sc3
3. 2a2>11
0-R
`
	if string(ptn) != want {
		t.Errorf("ptn =\n%s\nwant\n%s", ptn, want)
	}
	// Pre-2016 games are no longer anonymized.
	resp, _ = http.Get(srv.URL + "/v1/games-history/ptn/1")
	ptn, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(ptn), `[Player1 "alice"]`) || !strings.Contains(string(ptn), `[Clock "10:0 +10"]`) {
		t.Errorf("game 1 ptn =\n%s", ptn)
	}
}

func TestPTNHelpers(t *testing.T) {
	for in, want := range map[string]string{"P A4": "a4", "P A4 C": "Ca4", "P A4 W": "Sa4", "M A2 A5 2 1": "3a2+21", "M C3 A3 1": "1c3<1", "M C3 C1 2": "2c3-2", "M A1 B1 1": "1a1>1", "X": ""} {
		if got := ptnMove(in); got != want {
			t.Errorf("ptnMove(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[int]string{0: "0", 45: "45", 90: "1:30", 600: "10:0", 3600: "1:0:0", 5430: "1:30:30"} {
		if got := duration(in); got != want {
			t.Errorf("duration(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPublicGamesDBInfo(t *testing.T) {
	if code := getJSON(t, newPublicServer(t, "", ""), "/v1/games-history/db", nil); code != http.StatusNotFound {
		t.Errorf("unconfigured: %d, want 404", code)
	}
	path := filepath.Join(t.TempDir(), "games.db")
	os.WriteFile(path, make([]byte, 1234), 0o644)
	var info struct {
		Size  int64  `json:"size"`
		Mtime string `json:"mtime"`
	}
	if code := getJSON(t, newPublicServer(t, "", path), "/v1/games-history/db", &info); code != http.StatusOK || info.Size != 1234 || info.Mtime == "" {
		t.Errorf("info: %d %+v", code, info)
	}
}

func TestRatings(t *testing.T) {
	srv := newPublicServer(t, "", "")
	var list page[players.PublicRating]
	getJSON(t, srv, "/v1/ratings", &list)
	var names []string
	for _, r := range list.Items {
		names = append(names, r.Name)
	}
	// carol has no rated games and dave is unrated, so neither is listed.
	if !slices.Equal(names, []string{"alice", "bob", "Alice"}) || list.Total != 3 {
		t.Errorf("ratings = %v (total %d)", names, list.Total)
	}
	getJSON(t, srv, "/v1/ratings?name=ALICE&sort=rating&order=ASC", &list)
	if len(list.Items) != 2 || list.Items[0].Name != "Alice" {
		t.Errorf("name search = %+v", list.Items)
	}
	getJSON(t, srv, "/v1/ratings?limit=999999", &list)
	if list.PerPage != maxPageSize {
		t.Errorf("ratings perPage = %d, want the cap %d", list.PerPage, maxPageSize)
	}
	if code := getJSON(t, srv, "/v1/ratings?sort=email", nil); code != http.StatusBadRequest {
		t.Errorf("bad sort: %d", code)
	}

	var r players.PublicRating
	if code := getJSON(t, srv, "/v1/ratings/bob", &r); code != http.StatusOK || r.Rating != 1400.25 || !r.IsBot {
		t.Errorf("bob: %d %+v", code, r)
	}
	if code := getJSON(t, srv, "/v1/ratings/BOB", nil); code != http.StatusNotFound {
		t.Errorf("BOB: %d, want 404 (exact name)", code)
	}
}

const eventsCSV = `Event Name,Event Type,Tournament Type,Start Date,End Date,Details,Registration,Standings
League (Ongoing),Tournament,Special,"May 24, 2026",Late 2026,https://ustak.org/league,,https://example.com/standings
GenCon,Convention,Convention,"Jul 30, 2026","Aug 01, 2026",TBA,,
,,,,,,,
Beginner,Tournament,Special,"Sept 7, 2026",,https://ustak.org/b
`

func TestEvents(t *testing.T) {
	var fetches atomic.Int32
	var fail atomic.Bool
	sheet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if fail.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		io.WriteString(w, eventsCSV)
	}))
	defer sheet.Close()
	srv := newPublicServer(t, sheet.URL, "")

	var list eventList
	if code := getJSON(t, srv, "/events", &list); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(list.Data) != 3 || !slices.Equal(list.Categories, []string{"All", "Special", "Convention"}) {
		t.Fatalf("events = %+v", list)
	}
	league, gencon := list.Data[0], list.Data[1]
	if *league.StartDate != "May 24, 2026" || *league.Details != "https://ustak.org/league" || league.Registration != nil || *league.Standings != "https://example.com/standings" {
		t.Errorf("league = %+v", league)
	}
	if gencon.Details != nil || list.Data[2].EndDate != nil {
		t.Errorf("non-links and blanks should be null: %+v %+v", gencon, list.Data[2])
	}

	getJSON(t, srv, "/events", &list)
	if fetches.Load() != 1 {
		t.Errorf("fetched %d times, want 1 (cached)", fetches.Load())
	}
}

func TestEventsFallsBackToStaleCopy(t *testing.T) {
	var fail atomic.Bool
	sheet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		io.WriteString(w, eventsCSV)
	}))
	defer sheet.Close()
	feed := newEventsFeed(sheet.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := feed.get(t.Context()); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	feed.fetched = feed.fetched.Add(-2 * eventsCacheFor) // expire it
	list, err := feed.get(t.Context())
	if err != nil || len(list.Data) != 3 {
		t.Errorf("stale fallback = %v, %v", list, err)
	}

	empty := newEventsFeed(sheet.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := empty.get(t.Context()); err == nil {
		t.Error("no cache and a failing sheet should be an error")
	}
}

// While the sheet is down, requests don't each wait out a fetch: a failure
// is remembered for eventsRetryAfter.
func TestEventsFailureIsRemembered(t *testing.T) {
	var fetches atomic.Int32
	var fail atomic.Bool
	fail.Store(true)
	sheet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if fail.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		io.WriteString(w, eventsCSV)
	}))
	defer sheet.Close()
	feed := newEventsFeed(sheet.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for range 3 {
		if _, err := feed.get(t.Context()); err == nil {
			t.Fatal("want an error while the sheet is down")
		}
	}
	if n := fetches.Load(); n != 1 {
		t.Errorf("fetched %d times, want 1 within the retry window", n)
	}
	// After the window it tries again, and a success clears the failure.
	fail.Store(false)
	feed.failedAt = feed.failedAt.Add(-eventsRetryAfter)
	if list, err := feed.get(t.Context()); err != nil || len(list.Data) != 3 {
		t.Fatalf("after the window: %v, %v", list, err)
	}
	if !feed.failedAt.IsZero() || fetches.Load() != 2 {
		t.Errorf("failedAt %v, fetches %d", feed.failedAt, fetches.Load())
	}

	// With a stale copy, a failure serves it without refetching per request.
	fail.Store(true)
	feed.fetched = feed.fetched.Add(-2 * eventsCacheFor)
	for range 3 {
		if list, err := feed.get(t.Context()); err != nil || len(list.Data) != 3 {
			t.Fatalf("stale copy: %v, %v", list, err)
		}
	}
	if n := fetches.Load(); n != 3 {
		t.Errorf("fetched %d times, want 3", n)
	}
}

// A client hanging up doesn't abort the fetch other requests are waiting on.
func TestEventsFetchIgnoresRequestCancel(t *testing.T) {
	sheet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, eventsCSV)
	}))
	defer sheet.Close()
	feed := newEventsFeed(sheet.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if list, err := feed.get(ctx); err != nil || len(list.Data) != 3 {
		t.Errorf("fetch with a cancelled request: %v, %v", list, err)
	}
}
