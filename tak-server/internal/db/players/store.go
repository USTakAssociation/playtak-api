// Package players reads and writes the players database.
package players

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrNameTaken = errors.New("player name is already taken")
)

// Record is one row of the players table, limited to the columns the server
// uses for identity and permissions.
type Record struct {
	ID           int
	Name         string
	PasswordHash string
	Email        string
	IsBot        bool
	IsAdmin      bool
	IsMod        bool
	IsGagged     bool
	IsBanned     bool
}

// Flag is a boolean permission column on the players table.
type Flag string

const (
	FlagBot    Flag = "isbot"
	FlagAdmin  Flag = "is_admin"
	FlagMod    Flag = "is_mod"
	FlagGagged Flag = "is_gagged"
	FlagBanned Flag = "is_banned"
)

// normalizedNameSQL is the SQL form of NormalizeName for names made of
// [A-Za-z0-9_], which is all registration allows.
const normalizedNameSQL = `replace(replace(replace(replace(lower(name), '_', ''), 'i', '1'), 'l', '1'), 'o', '0')`

const selectRecord = `
	SELECT id, COALESCE(name, ''), COALESCE(password, ''), COALESCE(email, ''),
	       COALESCE(isbot, 0), COALESCE(is_admin, 0), COALESCE(is_mod, 0),
	       COALESCE(is_gagged, 0), COALESCE(is_banned, 0)
	FROM players`

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ByName returns the player with exactly this name (case-sensitive).
func (s *Store) ByName(ctx context.Context, name string) (Record, error) {
	return s.one(ctx, selectRecord+` WHERE name = ?`, name)
}

// ByNameFold returns the player with this name, ignoring case. Names are
// unique ignoring case.
func (s *Store) ByNameFold(ctx context.Context, name string) (Record, error) {
	return s.one(ctx, selectRecord+` WHERE name = ? COLLATE NOCASE`, name)
}

func (s *Store) ByID(ctx context.Context, id int) (Record, error) {
	return s.one(ctx, selectRecord+` WHERE id = ?`, id)
}

func (s *Store) one(ctx context.Context, query string, arg any) (Record, error) {
	var record Record
	err := s.db.QueryRowContext(ctx, query, arg).Scan(&record.ID, &record.Name, &record.PasswordHash, &record.Email,
		&record.IsBot, &record.IsAdmin, &record.IsMod, &record.IsGagged, &record.IsBanned)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("query player %v: %w", arg, err)
	}
	return record, nil
}

// NameTaken reports whether name, or a look-alike of it, is registered.
func (s *Store) NameTaken(ctx context.Context, name string) (bool, error) {
	var taken bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM players WHERE `+normalizedNameSQL+` = ?)`,
		NormalizeName(name)).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check name %s: %w", name, err)
	}
	return taken, nil
}

// Create registers a new player and returns it with its assigned ID. The
// look-alike check and the insert are one statement, so two concurrent
// registrations of similar names cannot both succeed.
func (s *Store) Create(ctx context.Context, name, email, passwordHash string) (Record, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO players (name, password, email)
		SELECT ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM players WHERE `+normalizedNameSQL+` = ?)`,
		name, passwordHash, email, NormalizeName(name))
	if err != nil {
		return Record{}, fmt.Errorf("create player %s: %w", name, err)
	}
	if n, err := result.RowsAffected(); err != nil {
		return Record{}, fmt.Errorf("create player %s: %w", name, err)
	} else if n == 0 {
		return Record{}, ErrNameTaken
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Record{}, fmt.Errorf("create player %s: %w", name, err)
	}
	return Record{ID: int(id), Name: name, PasswordHash: passwordHash, Email: email}, nil
}

func (s *Store) SetPassword(ctx context.Context, id int, hash string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE players SET password = ? WHERE id = ?`, hash, id); err != nil {
		return fmt.Errorf("set password for player %d: %w", id, err)
	}
	return nil
}

func (s *Store) SetFlag(ctx context.Context, name string, flag Flag, on bool) error {
	switch flag {
	case FlagBot, FlagAdmin, FlagMod, FlagGagged, FlagBanned:
	default:
		return fmt.Errorf("unknown player flag %q", flag)
	}
	// flag is one of the constants above, so formatting it into the query is safe.
	query := fmt.Sprintf(`UPDATE players SET %s = ? WHERE name = ?`, flag)
	if _, err := s.db.ExecContext(ctx, query, on, name); err != nil {
		return fmt.Errorf("set %s for player %s: %w", flag, name, err)
	}
	return nil
}

// Flagged returns the names of players with flag set, sorted ignoring case.
func (s *Store) Flagged(ctx context.Context, flag Flag) ([]string, error) {
	switch flag {
	case FlagBot, FlagAdmin, FlagMod, FlagGagged, FlagBanned:
	default:
		return nil, fmt.Errorf("unknown player flag %q", flag)
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM players WHERE %s = 1 ORDER BY name COLLATE NOCASE`, flag))
	if err != nil {
		return nil, fmt.Errorf("list %s players: %w", flag, err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("list %s players: %w", flag, err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list %s players: %w", flag, err)
	}
	return names, nil
}

// ratingDecay is the period over which an inactive high rating decays.
const ratingDecay = 240 * 24 * time.Hour

// Rating returns the rating recorded for player id when a game starts at
// time at, matching the Java server: 0 for unrated players and players
// without a row (guests), and for ratings of 1500 or more a decay that
// lowers ratings that haven't been played for. A player whose ratingbase is
// set uses that player's rating instead.
func (s *Store) Rating(ctx context.Context, id int, at time.Time) (int, error) {
	type ratingRow struct {
		rating, age float64
		base        int
		unrated     bool
	}
	// load reads id's rating fields into r, leaving r unchanged if there's
	// no such player.
	load := func(id int, r *ratingRow) error {
		err := s.db.QueryRowContext(ctx,
			`SELECT COALESCE(rating, 0), COALESCE(ratingage, 0), COALESCE(ratingbase, 0), COALESCE(unrated, 0) FROM players WHERE id = ?`,
			id).Scan(&r.rating, &r.age, &r.base, &r.unrated)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	row := ratingRow{unrated: true}
	if err := load(id, &row); err != nil {
		return 0, fmt.Errorf("load rating for player %d: %w", id, err)
	}
	if row.base != 0 {
		if err := load(row.base, &row); err != nil {
			return 0, fmt.Errorf("load base rating for player %d: %w", id, err)
		}
	}
	if row.unrated {
		return 0, nil
	}
	if row.rating < 1500 {
		return int(row.rating), nil
	}
	decay := (float64(at.UnixMilli()) - row.age) / float64(ratingDecay.Milliseconds())
	if decay < 1 {
		return int(row.rating), nil
	}
	retention := math.Pow(0.5, decay) * 2
	if row.rating < 1700 {
		return int(math.Min(row.rating, 1500+retention*200)), nil
	}
	return int(row.rating - 200*(1-retention)), nil
}

// CheckSchema verifies the database has had the migrations this package
// relies on, so a missed migration fails at startup rather than mid-request.
func CheckSchema(ctx context.Context, conn *sql.DB) error {
	var pk int
	err := conn.QueryRowContext(ctx, `SELECT pk FROM pragma_table_info('players') WHERE name = 'id'`).Scan(&pk)
	if err != nil {
		return fmt.Errorf("check players schema: %w", err)
	}
	if pk != 1 {
		return errors.New("players.id is not a primary key: run scripts/migrations/2026-09-players-id-primary-key.sh")
	}
	var hasTokens bool
	err = conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'auth_tokens')`).Scan(&hasTokens)
	if err != nil {
		return fmt.Errorf("check players schema: %w", err)
	}
	if !hasTokens {
		return errors.New("auth_tokens table is missing: run scripts/migrations/2026-09-add-auth-tokens.sh")
	}
	return nil
}

// NormalizeName folds case, drops underscores and maps look-alike characters
// so that e.g. "Bill_0" and "b1llo" count as the same name.
func NormalizeName(name string) string {
	return strings.NewReplacer("_", "", "i", "1", "l", "1", "o", "0").Replace(strings.ToLower(name))
}
