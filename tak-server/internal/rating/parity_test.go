package rating

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
)

// The fixture's run times; see the header comments in testdata/fixture*.sql.
var (
	now1 = time.UnixMilli(1767225600000)
	now2 = time.UnixMilli(1767312000000)
)

//go:embed testdata/fixture.sql
var fixture string

//go:embed testdata/fixture2.sql
var fixture2 string

// dump is everything the job writes, in the shape tests/rating-parity/dump.sh
// prints it.
type playerDump struct {
	ID            int      `json:"id"`
	Rating        float64  `json:"rating"`
	Boost         float64  `json:"boost"`
	RatedGames    int      `json:"ratedgames"`
	MaxRating     float64  `json:"maxrating"`
	RatingAge     float64  `json:"ratingage"`
	Participation *float64 `json:"participation_rating"`
	Fatigue       string   `json:"fatigue"`
}

type gameDump struct {
	ID          int `json:"id"`
	RatingWhite int `json:"rating_white"`
	RatingBlack int `json:"rating_black"`
	ChangeWhite int `json:"rating_change_white"`
	ChangeBlack int `json:"rating_change_black"`
}

type dump struct {
	Cursor     int          `json:"cursor"`
	Players    []playerDump `json:"players"`
	Games      []gameDump   `json:"games"`
	RatingList [][5]any     `json:"ratinglist"`
}

// TestMatchesNodeJob runs the fixture through the job twice and compares
// every written value with golden.json, the Node job's output for the same
// fixture (regenerated with tests/rating-parity/make-golden.sh, local-only
// tooling that needs the NestJS API; it is gitignored).
func TestMatchesNodeJob(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct{ Run1, Run2 dump }
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}

	env := newEnv(t)
	exec(t, env.games, fixture)
	env.run(t, now1)
	compare(t, "run 1", golden.Run1, env.dump(t))
	exec(t, env.games, fixture2)
	env.run(t, now2)
	compare(t, "run 2", golden.Run2, env.dump(t))
}

type env struct {
	dir   string
	games *sql.DB
	conn  *sql.Conn
}

func newEnv(t *testing.T) *env {
	t.Helper()
	playersDB := dbtest.NewPlayersDB(t)
	gamesDB := dbtest.NewGamesDB(t)
	var playersPath, gamesPath string
	playersDB.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&playersPath)
	gamesDB.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&gamesPath)

	ctx := context.Background()
	conn, err := gamesDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := AttachPlayers(ctx, conn, playersPath); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "previous.txt"), []byte("0 hash"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &env{dir: dir, games: gamesDB, conn: conn}
	// The fixture inserts players too; route them to the attached database.
	if _, err := conn.ExecContext(ctx, strings.ReplaceAll(fixturePlayers(), "INSERT INTO players", "INSERT INTO p.players")); err != nil {
		t.Fatal(err)
	}
	return e
}

// fixturePlayers is the players part of fixture.sql; exec runs the games
// part against the games database.
func fixturePlayers() string {
	var b strings.Builder
	for line := range strings.Lines(fixture) {
		if strings.HasPrefix(line, "INSERT INTO players") {
			b.WriteString(line)
		}
	}
	return b.String()
}

func exec(t *testing.T, conn *sql.DB, sqlText string) {
	t.Helper()
	var b strings.Builder
	for line := range strings.Lines(sqlText) {
		if !strings.HasPrefix(line, "INSERT INTO players") {
			b.WriteString(line)
		}
	}
	if _, err := conn.Exec(b.String()); err != nil {
		t.Fatal(err)
	}
}

func (e *env) files() Files {
	return Files{Cursor: filepath.Join(e.dir, "previous.txt"), RatingList: filepath.Join(e.dir, "ratinglist.json.gz")}
}

func (e *env) run(t *testing.T, now time.Time) Result {
	t.Helper()
	result, err := Run(context.Background(), e.conn, e.files(), now)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (e *env) dump(t *testing.T) dump {
	t.Helper()
	ctx := context.Background()
	var d dump
	cursor, err := readCursor(e.files().Cursor)
	if err != nil {
		t.Fatal(err)
	}
	d.Cursor = cursor

	rows, err := e.conn.QueryContext(ctx, `
		SELECT id, rating, boost, ratedgames, maxrating, ratingage, participation_rating, fatigue FROM p.players ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var p playerDump
		if err := rows.Scan(&p.ID, &p.Rating, &p.Boost, &p.RatedGames, &p.MaxRating, &p.RatingAge, &p.Participation, &p.Fatigue); err != nil {
			t.Fatal(err)
		}
		d.Players = append(d.Players, p)
	}
	rows.Close()

	rows, err = e.conn.QueryContext(ctx, `
		SELECT id, rating_white, rating_black, rating_change_white, rating_change_black FROM games ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var g gameDump
		if err := rows.Scan(&g.ID, &g.RatingWhite, &g.RatingBlack, &g.ChangeWhite, &g.ChangeBlack); err != nil {
			t.Fatal(err)
		}
		d.Games = append(d.Games, g)
	}
	rows.Close()

	gz, err := os.ReadFile(e.files().RatingList)
	if errors.Is(err, fs.ErrNotExist) {
		return d
	}
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	list, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(list, &d.RatingList); err != nil {
		t.Fatal(err)
	}
	return d
}

// closeEnough allows for last-bit differences between V8's and Go's pow and
// log2; anything a real divergence would cause is far larger.
func closeEnough(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func compare(t *testing.T, label string, want, got dump) {
	t.Helper()
	if want.Cursor != got.Cursor {
		t.Errorf("%s: cursor = %d, want %d", label, got.Cursor, want.Cursor)
	}
	if len(want.Players) != len(got.Players) || len(want.Games) != len(got.Games) {
		t.Fatalf("%s: %d players / %d games, want %d / %d", label, len(got.Players), len(got.Games), len(want.Players), len(want.Games))
	}
	for i, w := range want.Players {
		g := got.Players[i]
		floats := []struct {
			name     string
			want, gt float64
		}{{"rating", w.Rating, g.Rating}, {"boost", w.Boost, g.Boost}, {"maxrating", w.MaxRating, g.MaxRating}, {"ratingage", w.RatingAge, g.RatingAge}}
		for _, f := range floats {
			if !closeEnough(f.want, f.gt) {
				t.Errorf("%s: player %d %s = %v, want %v", label, w.ID, f.name, f.gt, f.want)
			}
		}
		if w.RatedGames != g.RatedGames {
			t.Errorf("%s: player %d ratedgames = %d, want %d", label, w.ID, g.RatedGames, w.RatedGames)
		}
		if (w.Participation == nil) != (g.Participation == nil) || (w.Participation != nil && !closeEnough(*w.Participation, *g.Participation)) {
			t.Errorf("%s: player %d participation_rating = %v, want %v", label, w.ID, deref(g.Participation), deref(w.Participation))
		}
		if !sameFatigue(w.Fatigue, g.Fatigue) {
			t.Errorf("%s: player %d fatigue = %s, want %s", label, w.ID, g.Fatigue, w.Fatigue)
		}
	}
	for i, w := range want.Games {
		if g := got.Games[i]; w != g {
			t.Errorf("%s: game %d = %+v, want %+v", label, w.ID, g, w)
		}
	}
	wantList, _ := json.Marshal(want.RatingList)
	gotList, _ := json.Marshal(got.RatingList)
	if !bytes.Equal(wantList, gotList) {
		t.Errorf("%s: rating list =\n%s\nwant\n%s", label, gotList, wantList)
	}
}

func sameFatigue(want, got string) bool {
	w, g := parseFatigue(want), parseFatigue(got)
	if len(w) != len(g) {
		return false
	}
	for k, v := range w {
		if gv, ok := g[k]; !ok || !closeEnough(v, gv) {
			return false
		}
	}
	return true
}

func deref(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

// TestReplay runs the job to completion over the databases in
// $RATING_REPLAY_DIR with the clock at $RATING_REPLAY_NOW (ms). It is for
// tests/rating-parity/replay.sh, which compares the result with the Node
// job's; it is skipped otherwise.
func TestReplay(t *testing.T) {
	dir := os.Getenv("RATING_REPLAY_DIR")
	if dir == "" {
		t.Skip("RATING_REPLAY_DIR not set")
	}
	nowMs, err := strconv.ParseInt(os.Getenv("RATING_REPLAY_NOW"), 10, 64)
	if err != nil {
		t.Fatal("RATING_REPLAY_NOW:", err)
	}
	ctx := context.Background()
	gamesDB, err := db.Open(ctx, filepath.Join(dir, "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer gamesDB.Close()
	conn, err := gamesDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := AttachPlayers(ctx, conn, filepath.Join(dir, "players.db")); err != nil {
		t.Fatal(err)
	}
	files := Files{Cursor: filepath.Join(dir, "previous.txt"), RatingList: filepath.Join(dir, "ratinglist.json.gz")}
	for run := 1; ; run++ {
		start := time.Now()
		result, err := Run(ctx, conn, files, time.UnixMilli(nowMs))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("run %d: cursor %d -> %d (%d games written) in %v", run, result.From, result.To, result.Games, time.Since(start).Round(time.Millisecond))
		if result.To == result.From {
			return
		}
	}
}
