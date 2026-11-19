package players

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
)

type TokenKind string

const (
	TokenAccess  TokenKind = "access"
	TokenRefresh TokenKind = "refresh"
)

// Token is a row of auth_tokens. Hash is the SHA-256 of the token handed to
// the client; the token itself is never stored.
type Token struct {
	Hash      []byte
	PlayerID  int
	Kind      TokenKind
	CreatedAt time.Time
	ExpiresAt time.Time
}

type TokenStore struct {
	db *sql.DB
}

func NewTokenStore(db *sql.DB) *TokenStore {
	return &TokenStore{db: db}
}

// Insert stores tokens atomically.
func (s *TokenStore) Insert(ctx context.Context, tokens ...Token) error {
	return db.WithTransaction(ctx, s.db, func(tx *sql.Tx) error {
		return insertTokens(ctx, tx, tokens)
	})
}

// Get returns the token with the given hash if it has not expired at now.
func (s *TokenStore) Get(ctx context.Context, hash []byte, now time.Time) (Token, error) {
	t := Token{Hash: hash}
	var kind string
	var created, expires int64
	err := s.db.QueryRowContext(ctx,
		`SELECT player_id, kind, created_at, expires_at FROM auth_tokens WHERE token_hash = ? AND expires_at > ?`,
		hash, now.Unix()).Scan(&t.PlayerID, &kind, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, ErrNotFound
	}
	if err != nil {
		return Token{}, fmt.Errorf("get token: %w", err)
	}
	t.Kind = TokenKind(kind)
	t.CreatedAt = time.Unix(created, 0)
	t.ExpiresAt = time.Unix(expires, 0)
	return t, nil
}

// Rotate deletes the unexpired refresh token oldHash and stores next in its
// place, atomically. It returns ErrNotFound if oldHash was already used,
// revoked or expired, so a refresh token can only be exchanged once.
func (s *TokenStore) Rotate(ctx context.Context, oldHash []byte, now time.Time, next ...Token) error {
	return db.WithTransaction(ctx, s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM auth_tokens WHERE token_hash = ? AND kind = ? AND expires_at > ?`,
			oldHash, TokenRefresh, now.Unix())
		if err != nil {
			return fmt.Errorf("delete refresh token: %w", err)
		}
		if n, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("delete refresh token: %w", err)
		} else if n == 0 {
			return ErrNotFound
		}
		return insertTokens(ctx, tx, next)
	})
}

// Delete removes the tokens with the given hashes. Unknown hashes are ignored.
func (s *TokenStore) Delete(ctx context.Context, hashes ...[]byte) error {
	return db.WithTransaction(ctx, s.db, func(tx *sql.Tx) error {
		for _, h := range hashes {
			if _, err := tx.ExecContext(ctx, `DELETE FROM auth_tokens WHERE token_hash = ?`, h); err != nil {
				return fmt.Errorf("delete token: %w", err)
			}
		}
		return nil
	})
}

// DeleteForPlayer revokes every token belonging to playerID.
func (s *TokenStore) DeleteForPlayer(ctx context.Context, playerID int) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE player_id = ?`, playerID)
	if err != nil {
		return 0, fmt.Errorf("delete tokens for player %d: %w", playerID, err)
	}
	return result.RowsAffected()
}

// DeleteExpired removes tokens that expired at or before now.
func (s *TokenStore) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("delete expired tokens: %w", err)
	}
	return result.RowsAffected()
}

func insertTokens(ctx context.Context, tx *sql.Tx, tokens []Token) error {
	for _, t := range tokens {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO auth_tokens (token_hash, player_id, kind, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
			t.Hash, t.PlayerID, t.Kind, t.CreatedAt.Unix(), t.ExpiresAt.Unix())
		if err != nil {
			return fmt.Errorf("insert %s token for player %d: %w", t.Kind, t.PlayerID, err)
		}
	}
	return nil
}
