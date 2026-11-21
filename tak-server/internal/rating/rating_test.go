package rating

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSRound(t *testing.T) {
	for x, want := range map[float64]int{2.5: 3, -2.5: -2, -2.6: -3, 0.49999999999999994: 0, -0.5: 0, 7: 7} {
		if got := jsRound(x); got != want {
			t.Errorf("jsRound(%v) = %d, want %d", x, got, want)
		}
	}
}

func TestParseFatigue(t *testing.T) {
	for raw, want := range map[string]int{
		`{"85":0.2,"9":1}`:       2,
		`"{\"85\":0.2,\"9\":1}"`: 2, // written by the Node job
		`not json`:               0, // Node crashed the whole run on this
		`null`:                   0,
		`{}`:                     0,
	} {
		got := parseFatigue(raw)
		if got == nil || len(got) != want {
			t.Errorf("parseFatigue(%q) = %v, want %d entries", raw, got, want)
		}
	}
}

func TestMissingCursorIsAnError(t *testing.T) {
	env := newEnv(t)
	os.Remove(env.files().Cursor)
	if _, err := Run(context.Background(), env.conn, env.files(), now1); err == nil {
		t.Fatal("Run without a cursor file succeeded; it would re-rate every game")
	}
}

// A failure partway through writing leaves both databases and the cursor as
// they were, so the next run redoes the same games from the same state.
func TestFailedWriteChangesNothing(t *testing.T) {
	env := newEnv(t)
	exec(t, env.games, fixture)
	ctx := context.Background()
	if _, err := env.conn.ExecContext(ctx, `
		CREATE TRIGGER p.fail_player_update BEFORE UPDATE ON players WHEN NEW.id = 12
		BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := env.dump(t)

	_, err := Run(ctx, env.conn, env.files(), now1)
	if err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("Run error = %v, want the injected failure", err)
	}
	after := env.dump(t)
	if len(before.Games) != len(after.Games) {
		t.Fatal("game count changed")
	}
	for i := range before.Games {
		if before.Games[i] != after.Games[i] {
			t.Fatalf("game %d changed after a failed run: %+v -> %+v", before.Games[i].ID, before.Games[i], after.Games[i])
		}
	}
	for i := range before.Players {
		if before.Players[i].Rating != after.Players[i].Rating || before.Players[i].RatedGames != after.Players[i].RatedGames {
			t.Fatalf("player %d changed after a failed run", before.Players[i].ID)
		}
	}
	if after.Cursor != 0 {
		t.Fatalf("cursor = %d after a failed run, want 0", after.Cursor)
	}
	if _, err := os.Stat(filepath.Join(env.dir, "ratinglist.json.gz")); err == nil {
		t.Fatal("rating list written by a failed run")
	}
}
