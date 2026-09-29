package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
)

func TestTokenPairLifecycle(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.addPlayer(t, "alice", "", "secret123")

	pair, err := f.svc.IssueTokens(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.svc.Authenticate(ctx, pair.AccessToken); err != nil || got.ID != alice.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	if _, err := f.svc.Authenticate(ctx, pair.RefreshToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("refresh token accepted as access token: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, pair.AccessToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("access token accepted as refresh token: %v", err)
	}

	next, err := f.svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if next.AccessToken == pair.AccessToken || next.RefreshToken == pair.RefreshToken {
		t.Error("refresh must issue new tokens")
	}
	if _, err := f.svc.Refresh(ctx, pair.RefreshToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("refresh token reused: %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, next.AccessToken); err != nil {
		t.Errorf("new access token: %v", err)
	}

	if err := f.svc.Logout(ctx, next.AccessToken, next.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, next.AccessToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("access token works after logout: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, next.RefreshToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("refresh token works after logout: %v", err)
	}
}

func TestAccessTokenExpires(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.addPlayer(t, "alice", "", "secret123")
	pair, _ := f.svc.IssueTokens(ctx, alice.ID)

	f.svc.now = func() time.Time { return time.Now().Add(AccessTokenTTL + time.Minute) }
	if _, err := f.svc.Authenticate(ctx, pair.AccessToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("expired access token: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, pair.RefreshToken); err != nil {
		t.Errorf("refresh token should still work: %v", err)
	}
}

func TestBannedPlayerTokensStopWorking(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.addPlayer(t, "alice", "", "secret123")
	pair, _ := f.svc.IssueTokens(ctx, alice.ID)

	f.store.SetFlag(ctx, "alice", players.FlagBanned, true)
	if _, err := f.svc.Authenticate(ctx, pair.AccessToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("banned player authenticated: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, pair.RefreshToken); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("banned player refreshed: %v", err)
	}
}

func TestRevokeAllAndPurge(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.addPlayer(t, "alice", "", "secret123")
	f.svc.IssueTokens(ctx, alice.ID)
	f.svc.IssueTokens(ctx, alice.ID)

	if n, err := f.svc.RevokeAll(ctx, "alice"); err != nil || n != 4 {
		t.Errorf("RevokeAll = %d, %v; want 4", n, err)
	}
	if _, err := f.svc.RevokeAll(ctx, "nobody"); !errors.Is(err, ErrNoSuchPlayer) {
		t.Errorf("RevokeAll unknown: %v", err)
	}

	f.svc.IssueTokens(ctx, alice.ID)
	f.svc.now = func() time.Time { return time.Now().Add(2 * AccessTokenTTL) }
	if n, err := f.svc.PurgeExpiredTokens(ctx); err != nil || n != 1 {
		t.Errorf("PurgeExpiredTokens = %d, %v; want 1 (the access token)", n, err)
	}
}
