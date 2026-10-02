package players

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/dbtest"
)

func newTestDB(t *testing.T) *sql.DB { return dbtest.NewPlayersDB(t) }

func TestRating(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	day := int64(24 * 60 * 60 * 1000)
	now := time.UnixMilli(1000 * 240 * day) // far enough from 0 for ages below
	nowMs := now.UnixMilli()
	for _, r := range []struct {
		id      int
		rating  float64
		age     int64
		base    int
		unrated int
	}{
		{1, 1200, 0, 0, 0},                // below 1500: no decay
		{2, 1800, nowMs - 100*day, 0, 0},  // played recently: no decay
		{3, 1800, nowMs - 480*day, 0, 0},  // two decay periods: 1800 - 200*(1-0.5)
		{4, 1600, nowMs - 480*day, 0, 0},  // 1500-1700: capped at 1500 + 0.5*200
		{5, 1600, nowMs - 2400*day, 0, 0}, // long gone: falls to just above 1500
		{6, 1900, 0, 0, 1},                // unrated
		{7, 0, 0, 1, 0},                   // uses player 1's rating
		{8, 1750, 0, 999, 0},              // base missing: keeps its own values (and decays)
	} {
		if _, err := conn.Exec(`INSERT INTO players (id, name, rating, ratingage, ratingbase, unrated) VALUES (?, ?, ?, ?, ?, ?)`,
			r.id, fmt.Sprint("p", r.id), r.rating, r.age, r.base, r.unrated); err != nil {
			t.Fatal(err)
		}
	}
	s := NewStore(conn)
	for id, want := range map[int]int{1: 1200, 2: 1800, 3: 1700, 4: 1600, 5: 1500, 6: 0, 7: 1200, 8: 1550, 42: 0} {
		got, err := s.Rating(ctx, id, now)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Rating(%d) = %d, want %d", id, got, want)
		}
	}
}

func TestCheckSchema(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	if err := CheckSchema(ctx, conn); err != nil {
		t.Fatalf("migrated schema rejected: %v", err)
	}
	if _, err := conn.Exec(`DROP TABLE auth_tokens`); err != nil {
		t.Fatal(err)
	}
	if err := CheckSchema(ctx, conn); err == nil {
		t.Error("missing auth_tokens not detected")
	}
}

func TestCreateAndLookup(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newTestDB(t))

	a, err := s.Create(ctx, "alice", "a@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create(ctx, "bob", "b@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 || b.ID != a.ID+1 {
		t.Errorf("ids = %d, %d", a.ID, b.ID)
	}

	got, err := s.ByName(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got != a {
		t.Errorf("ByName = %+v, want %+v", got, a)
	}
	if got, err := s.ByID(ctx, b.ID); err != nil || got.Name != "bob" {
		t.Errorf("ByID = %+v, %v", got, err)
	}
	if _, err := s.ByName(ctx, "Alice"); !errors.Is(err, ErrNotFound) {
		t.Errorf("lookups must be case-sensitive, got err %v", err)
	}
}

func TestLookAlikeNames(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newTestDB(t))
	if _, err := s.Create(ctx, "Bill_0", "", ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bill0", "B1LL0", "bi_ll_o", "BiIlO"} {
		taken, err := s.NameTaken(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if !taken {
			t.Errorf("%q should be taken", name)
		}
		if _, err := s.Create(ctx, name, "", ""); !errors.Is(err, ErrNameTaken) {
			t.Errorf("Create(%q) err = %v, want ErrNameTaken", name, err)
		}
	}
	if taken, _ := s.NameTaken(ctx, "william"); taken {
		t.Error("william should be free")
	}
}

func TestConcurrentCreateSameName(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newTestDB(t))
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for range 10 {
		wg.Go(func() {
			if _, err := s.Create(ctx, "racer", "", ""); err == nil {
				mu.Lock()
				created++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if created != 1 {
		t.Errorf("created %d players named racer, want 1", created)
	}
}

func TestNullColumnsReadAsZero(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	if _, err := conn.Exec(`INSERT INTO players (id, name, is_mod) VALUES (5, 'legacy', NULL)`); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(conn).ByName(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Record{ID: 5, Name: "legacy"}) {
		t.Errorf("got %+v", got)
	}
}

func TestSetPasswordAndFlags(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newTestDB(t))
	p, _ := s.Create(ctx, "bob", "", "old")
	if err := s.SetPassword(ctx, p.ID, "newhash"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []Flag{FlagBot, FlagAdmin, FlagMod, FlagGagged, FlagBanned} {
		if err := s.SetFlag(ctx, "bob", f, true); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.ByName(ctx, "bob")
	want := Record{ID: p.ID, Name: "bob", PasswordHash: "newhash",
		IsBot: true, IsAdmin: true, IsMod: true, IsGagged: true, IsBanned: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if err := s.SetFlag(ctx, "bob", Flag("password"), true); err == nil {
		t.Error("SetFlag must reject unknown columns")
	}
}

func TestNormalizeNameMatchesSQL(t *testing.T) {
	conn := newTestDB(t)
	for _, name := range []string{"Bill_0", "LOL_ili", "Abc_123", "o_O"} {
		var sqlForm string
		if err := conn.QueryRow(`SELECT `+normalizedNameSQL+` FROM (SELECT ? AS name)`, name).Scan(&sqlForm); err != nil {
			t.Fatal(err)
		}
		if got := NormalizeName(name); got != sqlForm {
			t.Errorf("NormalizeName(%q) = %q, SQL = %q", name, got, sqlForm)
		}
	}
}
