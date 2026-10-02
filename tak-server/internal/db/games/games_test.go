package games

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
)

func TestInsertAndFinish(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.NewGamesDB(t)
	s := NewStore(conn)
	date := time.UnixMilli(1_800_000_000_123)
	id, err := s.Insert(ctx, Row{
		Date: date, Size: 6, White: "alice", Black: "bobby", Time: 600, Increment: 30,
		RatingWhite: 1612, RatingBlack: 0, Tournament: true, Komi: 4, Pieces: 30, Capstones: 1,
		ExtraTimeAmount: 300, ExtraTimeTrigger: 20, IncrementScales: true, Opening: "double black stack",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Errorf("id %d", id)
	}

	type row struct {
		date                                      int64
		size                                      int
		white, black, notation, result, opening   string
		time, inc, rw, rb, unrated, tour, komi    int
		pieces, caps, rcw, rcb, amount, trig, inS int
	}
	read := func() row {
		var r row
		err := conn.QueryRow(`SELECT date, size, player_white, player_black, notation, result, opening,
			timertime, timerinc, rating_white, rating_black, unrated, tournament, komi,
			pieces, capstones, rating_change_white, rating_change_black, extra_time_amount, extra_time_trigger, increment_scales
			FROM games WHERE id = ?`, id).Scan(&r.date, &r.size, &r.white, &r.black, &r.notation, &r.result, &r.opening,
			&r.time, &r.inc, &r.rw, &r.rb, &r.unrated, &r.tour, &r.komi,
			&r.pieces, &r.caps, &r.rcw, &r.rcb, &r.amount, &r.trig, &r.inS)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	got := read()
	want := row{
		date: 1_800_000_000_123, size: 6, white: "alice", black: "bobby", notation: "", result: "0-0",
		opening: "double black stack", time: 600, inc: 30, rw: 1612, rb: 0, unrated: 0, tour: 1, komi: 4,
		pieces: 30, caps: 1, rcw: -1000, rcb: -1000, amount: 300, trig: 20, inS: 1,
	}
	if got != want {
		t.Errorf("inserted row\n got %+v\nwant %+v", got, want)
	}

	if err := s.Finish(ctx, id, "P A1,P F6,M A1 A2 1", "R-0"); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.notation != "P A1,P F6,M A1 A2 1" || got.result != "R-0" {
		t.Errorf("after finish: %q %q", got.notation, got.result)
	}
}

// fakeFinisher records results and can be made to fail.
type fakeFinisher struct {
	mu    sync.Mutex
	fail  bool
	saved []Result
	calls chan struct{}
}

func (f *fakeFinisher) Finish(_ context.Context, id int, notation, result string) error {
	defer func() {
		select {
		case f.calls <- struct{}{}:
		default:
		}
	}()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("database is locked")
	}
	f.saved = append(f.saved, Result{id, notation, result})
	return nil
}

func (f *fakeFinisher) setFail(v bool) { f.mu.Lock(); f.fail = v; f.mu.Unlock() }
func (f *fakeFinisher) count() int     { f.mu.Lock(); defer f.mu.Unlock(); return len(f.saved) }

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestResultsSavedAndFileRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.jsonl")
	store := &fakeFinisher{calls: make(chan struct{}, 10)}
	r := NewResults(store, path, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx)

	if err := r.Save(Result{ID: 7, Notation: "P A1", Result: "0-1"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return store.count() == 1 })
	waitFor(t, func() bool { _, err := os.Stat(path); return errors.Is(err, os.ErrNotExist) })
	if store.saved[0] != (Result{7, "P A1", "0-1"}) {
		t.Errorf("saved %+v", store.saved[0])
	}
}

func TestResultsSurviveFailureAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.jsonl")
	store := &fakeFinisher{fail: true, calls: make(chan struct{}, 10)}
	r := NewResults(store, path, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	r.Save(Result{ID: 1, Notation: "P A1", Result: "R-0"})
	r.Save(Result{ID: 2, Notation: "P B1", Result: "0-R"})
	<-store.calls // at least one failed attempt
	cancel()
	<-done

	// The server "restarts" with a working database.
	store.setFail(false)
	r2 := NewResults(store, path, quiet)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	go r2.Run(ctx2)
	waitFor(t, func() bool { return store.count() == 2 })
	waitFor(t, func() bool { _, err := os.Stat(path); return errors.Is(err, os.ErrNotExist) })
}

func TestResultsKeepFailedOnlyAndSkipTornLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.jsonl")
	// A crash mid-append can leave a partial last line.
	os.WriteFile(path, []byte(`{"id":1,"notation":"P A1","result":"R-0"}`+"\n"+`{"id":2,"nota`), 0o644)
	store := &fakeFinisher{calls: make(chan struct{}, 10)}
	r := NewResults(store, path, quiet)
	if left := r.flush(context.Background()); left != 0 {
		t.Errorf("left %d", left)
	}
	if store.count() != 1 || store.saved[0].ID != 1 {
		t.Errorf("saved %+v", store.saved)
	}
}
