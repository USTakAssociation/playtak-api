package players

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PublicRating is a player's rating as the ratings API shows it. The JSON
// names are the column names the API has always returned.
type PublicRating struct {
	Name          string   `json:"name"`
	Rating        float64  `json:"rating"`
	RatedGames    int      `json:"ratedgames"`
	MaxRating     float64  `json:"maxrating"`
	Participation *float64 `json:"participation_rating"`
	IsBot         bool     `json:"isbot"`
}

const selectPublicRating = `
	SELECT COALESCE(name, ''), COALESCE(rating, 0), COALESCE(ratedgames, 0), COALESCE(maxrating, 0), participation_rating, COALESCE(isbot, 0)
	FROM players`

// RatingSearch selects and orders players for the ratings listing, which
// only includes rated players with at least one rated game.
type RatingSearch struct {
	Name string // exact, ignoring case
	ID   int
	// Sort is a column from RatingSortColumns.
	Sort          string
	Desc          bool
	Limit, Offset int
}

var RatingSortColumns = map[string]bool{
	"id": true, "name": true, "rating": true, "ratedgames": true, "maxrating": true, "participation_rating": true, "isbot": true,
}

// Ratings returns one page of the ratings listing and its total size.
func (s *Store) Ratings(ctx context.Context, q RatingSearch) ([]PublicRating, int, error) {
	if !RatingSortColumns[q.Sort] {
		return nil, 0, fmt.Errorf("ratings: cannot sort by %q", q.Sort)
	}
	where := ` WHERE ratingbase = 0 AND unrated = 0 AND ratedgames > 0`
	var args []any
	if q.Name != "" {
		where += ` AND name = ? COLLATE NOCASE`
		args = append(args, q.Name)
	}
	if q.ID != 0 {
		where += ` AND id = ?`
		args = append(args, q.ID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM players`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ratings: %w", err)
	}
	order := "ASC"
	if q.Desc {
		order = "DESC"
	}
	rows, err := s.db.QueryContext(ctx, selectPublicRating+where+` ORDER BY `+q.Sort+` `+order+` LIMIT ? OFFSET ?`,
		append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list ratings: %w", err)
	}
	defer rows.Close()
	ratings := []PublicRating{}
	for rows.Next() {
		var r PublicRating
		if err := rows.Scan(&r.Name, &r.Rating, &r.RatedGames, &r.MaxRating, &r.Participation, &r.IsBot); err != nil {
			return nil, 0, fmt.Errorf("list ratings: %w", err)
		}
		ratings = append(ratings, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list ratings: %w", err)
	}
	return ratings, total, nil
}

// RatingByName returns the rating of the player with exactly this name
// (case-sensitive), or ErrNotFound.
func (s *Store) RatingByName(ctx context.Context, name string) (PublicRating, error) {
	var r PublicRating
	err := s.db.QueryRowContext(ctx, selectPublicRating+` WHERE name = ?`, name).
		Scan(&r.Name, &r.Rating, &r.RatedGames, &r.MaxRating, &r.Participation, &r.IsBot)
	if errors.Is(err, sql.ErrNoRows) {
		return PublicRating{}, ErrNotFound
	}
	if err != nil {
		return PublicRating{}, fmt.Errorf("get rating for %s: %w", name, err)
	}
	return r, nil
}
