package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
)

const (
	AccessTokenTTL  = time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour
)

// TokenPair is what a client receives on login or refresh. The access token
// authenticates requests; the refresh token can be exchanged once for a new
// pair.
type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// IssueTokens creates a new token pair for a registered player.
func (s *Service) IssueTokens(ctx context.Context, playerID int) (TokenPair, error) {
	pair, rows := s.newPair(playerID)
	if err := s.tokens.Insert(ctx, rows...); err != nil {
		return TokenPair{}, err
	}
	return pair, nil
}

// Refresh exchanges an unused refresh token for a new pair, returning the
// player it belongs to as well. The old refresh token stops working; its
// access token lives out its short lifetime.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (players.Record, TokenPair, error) {
	now := s.now()
	old, err := s.tokens.Get(ctx, hashToken(refreshToken), now)
	if errors.Is(err, players.ErrNotFound) || (err == nil && old.Kind != players.TokenRefresh) {
		return players.Record{}, TokenPair{}, ErrAuthFailed
	}
	if err != nil {
		return players.Record{}, TokenPair{}, err
	}
	record, err := s.activePlayer(ctx, old.PlayerID)
	if err != nil {
		return players.Record{}, TokenPair{}, err
	}
	pair, rows := s.newPair(old.PlayerID)
	err = s.tokens.Rotate(ctx, hashToken(refreshToken), now, rows...)
	if errors.Is(err, players.ErrNotFound) {
		return players.Record{}, TokenPair{}, ErrAuthFailed // used concurrently by another request
	}
	if err != nil {
		return players.Record{}, TokenPair{}, err
	}
	return record, pair, nil
}

// Authenticate returns the player an access token belongs to.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (players.Record, error) {
	t, err := s.tokens.Get(ctx, hashToken(accessToken), s.now())
	if errors.Is(err, players.ErrNotFound) || (err == nil && t.Kind != players.TokenAccess) {
		return players.Record{}, ErrAuthFailed
	}
	if err != nil {
		return players.Record{}, err
	}
	return s.activePlayer(ctx, t.PlayerID)
}

// Logout revokes the given tokens. Unknown or empty tokens are ignored.
func (s *Service) Logout(ctx context.Context, tokens ...string) error {
	var hashes [][]byte
	for _, t := range tokens {
		if t != "" {
			hashes = append(hashes, hashToken(t))
		}
	}
	return s.tokens.Delete(ctx, hashes...)
}

// RevokeAll revokes every token a player holds and returns how many there were.
func (s *Service) RevokeAll(ctx context.Context, name string) (int64, error) {
	record, err := s.players.ByName(ctx, name)
	if errors.Is(err, players.ErrNotFound) {
		return 0, ErrNoSuchPlayer
	}
	if err != nil {
		return 0, err
	}
	return s.tokens.DeleteForPlayer(ctx, record.ID)
}

// PurgeExpiredTokens deletes expired tokens and returns how many there were.
func (s *Service) PurgeExpiredTokens(ctx context.Context) (int64, error) {
	return s.tokens.DeleteExpired(ctx, s.now())
}

// activePlayer loads a token's player, failing if they no longer exist or
// have been banned since the token was issued.
func (s *Service) activePlayer(ctx context.Context, id int) (players.Record, error) {
	record, err := s.players.ByID(ctx, id)
	if errors.Is(err, players.ErrNotFound) {
		return players.Record{}, ErrAuthFailed
	}
	if err != nil {
		return players.Record{}, fmt.Errorf("load player %d: %w", id, err)
	}
	if record.IsBanned {
		return players.Record{}, ErrAuthFailed
	}
	return record, nil
}

func (s *Service) newPair(playerID int) (TokenPair, []players.Token) {
	now := s.now()
	pair := TokenPair{
		AccessToken:      newToken(),
		AccessExpiresAt:  now.Add(AccessTokenTTL),
		RefreshToken:     newToken(),
		RefreshExpiresAt: now.Add(RefreshTokenTTL),
	}
	return pair, []players.Token{
		{Hash: hashToken(pair.AccessToken), PlayerID: playerID, Kind: players.TokenAccess, CreatedAt: now, ExpiresAt: pair.AccessExpiresAt},
		{Hash: hashToken(pair.RefreshToken), PlayerID: playerID, Kind: players.TokenRefresh, CreatedAt: now, ExpiresAt: pair.RefreshExpiresAt},
	}
}

func newToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
