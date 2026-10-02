// Package games reads and writes the games database.
package games

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
)

// Row is a game as recorded when it starts. Times are in seconds.
type Row struct {
	Date             time.Time
	Size             int
	White, Black     string
	Time, Increment  int
	RatingWhite      int
	RatingBlack      int
	Unrated          bool
	Tournament       bool
	Komi             int // half-flats
	Pieces           int
	Capstones        int
	ExtraTimeAmount  int
	ExtraTimeTrigger int
	IncrementScales  bool
	Opening          string // PTN name, e.g. "swap"
}

// insertBusyWait bounds how long Insert retries while another process (the
// hourly rating job) holds the write lock.
const insertBusyWait = 15 * time.Second

type Store struct {
	db *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{db: conn}
}

// Insert records a game that has just started and returns its ID.
//
// The row's result is "0-0" with empty notation and rating changes of -1000
// until Finish runs. The NestJS rating job relies on exactly these values to
// recognise games still in progress.
func (s *Store) Insert(ctx context.Context, r Row) (int, error) {
	var id int64
	err := db.RetryBusy(ctx, insertBusyWait, func() error {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO games (date, size, player_white, player_black, timertime, timerinc, notation, result,
				rating_white, rating_black, unrated, tournament, komi, pieces, capstones,
				rating_change_white, rating_change_black, extra_time_amount, extra_time_trigger, increment_scales, opening)
			VALUES (?, ?, ?, ?, ?, ?, '', '0-0', ?, ?, ?, ?, ?, ?, ?, -1000, -1000, ?, ?, ?, ?)`,
			r.Date.UnixMilli(), r.Size, r.White, r.Black, r.Time, r.Increment,
			r.RatingWhite, r.RatingBlack, r.Unrated, r.Tournament, r.Komi, r.Pieces, r.Capstones,
			r.ExtraTimeAmount, r.ExtraTimeTrigger, r.IncrementScales, r.Opening)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("insert game %s vs %s: %w", r.White, r.Black, err)
	}
	return int(id), nil
}

// Finish records a game's moves (comma-separated protocol notation) and result.
func (s *Store) Finish(ctx context.Context, id int, notation, result string) error {
	err := db.RetryBusy(ctx, 5*time.Second, func() error {
		_, err := s.db.ExecContext(ctx, `UPDATE games SET notation = ?, result = ? WHERE id = ?`, notation, result, id)
		return err
	})
	if err != nil {
		return fmt.Errorf("finish game %d: %w", id, err)
	}
	return nil
}
