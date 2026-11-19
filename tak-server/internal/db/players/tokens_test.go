package players

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	p, err := NewStore(conn).Create(ctx, "alice", "", "")
	if err != nil {
		t.Fatal(err)
	}
	tokens := NewTokenStore(conn)
	now := time.Unix(1_800_000_000, 0)

	access := Token{Hash: []byte("a1"), PlayerID: p.ID, Kind: TokenAccess, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	refresh := Token{Hash: []byte("r1"), PlayerID: p.ID, Kind: TokenRefresh, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := tokens.Insert(ctx, access, refresh); err != nil {
		t.Fatal(err)
	}

	got, err := tokens.Get(ctx, []byte("a1"), now)
	if err != nil {
		t.Fatal(err)
	}
	if got.PlayerID != p.ID || got.Kind != TokenAccess || !got.ExpiresAt.Equal(access.ExpiresAt) {
		t.Errorf("Get = %+v", got)
	}
	if _, err := tokens.Get(ctx, []byte("a1"), now.Add(2*time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired token: err = %v", err)
	}

	next := Token{Hash: []byte("r2"), PlayerID: p.ID, Kind: TokenRefresh, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := tokens.Rotate(ctx, []byte("r1"), now, next); err != nil {
		t.Fatal(err)
	}
	if err := tokens.Rotate(ctx, []byte("r1"), now, next); !errors.Is(err, ErrNotFound) {
		t.Errorf("reusing a refresh token: err = %v, want ErrNotFound", err)
	}
	if err := tokens.Rotate(ctx, []byte("a1"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("rotating an access token: err = %v, want ErrNotFound", err)
	}

	if err := tokens.Delete(ctx, []byte("r2"), []byte("unknown")); err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Get(ctx, []byte("r2"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted token still present: %v", err)
	}
}

func TestDeleteExpiredAndForPlayer(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	store := NewStore(conn)
	a, _ := store.Create(ctx, "alice", "", "")
	b, _ := store.Create(ctx, "bob", "", "")
	tokens := NewTokenStore(conn)
	now := time.Unix(1_800_000_000, 0)

	err := tokens.Insert(ctx,
		Token{Hash: []byte("old"), PlayerID: a.ID, Kind: TokenAccess, CreatedAt: now, ExpiresAt: now.Add(-time.Minute)},
		Token{Hash: []byte("a"), PlayerID: a.ID, Kind: TokenAccess, CreatedAt: now, ExpiresAt: now.Add(time.Hour)},
		Token{Hash: []byte("b"), PlayerID: b.ID, Kind: TokenAccess, CreatedAt: now, ExpiresAt: now.Add(time.Hour)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := tokens.DeleteExpired(ctx, now); err != nil || n != 1 {
		t.Errorf("DeleteExpired = %d, %v; want 1", n, err)
	}
	if n, err := tokens.DeleteForPlayer(ctx, a.ID); err != nil || n != 1 {
		t.Errorf("DeleteForPlayer = %d, %v; want 1", n, err)
	}
	if _, err := tokens.Get(ctx, []byte("b"), now); err != nil {
		t.Errorf("bob's token should remain: %v", err)
	}
}

func TestTokenRequiresExistingPlayer(t *testing.T) {
	ctx := context.Background()
	tokens := NewTokenStore(newTestDB(t))
	now := time.Now()
	err := tokens.Insert(ctx, Token{Hash: []byte("x"), PlayerID: 999, Kind: TokenAccess, CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err == nil {
		t.Fatal("foreign key should reject tokens for unknown players")
	}
}
