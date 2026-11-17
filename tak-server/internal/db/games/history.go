package games

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

var ErrNotFound = errors.New("game not found")

// Game is a full games row. The JSON names are the column names, which the
// games history API has always returned, so the row is served as-is.
type Game struct {
	ID                int    `json:"id"`
	Date              int64  `json:"date"` // ms timestamp
	Size              int    `json:"size"`
	PlayerWhite       string `json:"player_white"`
	PlayerBlack       string `json:"player_black"`
	Notation          string `json:"notation"`
	Result            string `json:"result"`
	TimerTime         int    `json:"timertime"`
	TimerInc          int    `json:"timerinc"`
	RatingWhite       int    `json:"rating_white"`
	RatingBlack       int    `json:"rating_black"`
	Unrated           int    `json:"unrated"`
	Tournament        int    `json:"tournament"`
	Komi              int    `json:"komi"`
	Pieces            int    `json:"pieces"`
	Capstones         int    `json:"capstones"`
	RatingChangeWhite int    `json:"rating_change_white"`
	RatingChangeBlack int    `json:"rating_change_black"`
	ExtraTimeAmount   int    `json:"extra_time_amount"`
	ExtraTimeTrigger  int    `json:"extra_time_trigger"`
	IncrementScales   int    `json:"increment_scales"`
	Opening           string `json:"opening"`
}

const gameColumns = `id, date, size, player_white, player_black, notation, result, timertime, timerinc,
	rating_white, rating_black, unrated, tournament, komi, pieces, capstones,
	rating_change_white, rating_change_black, extra_time_amount, extra_time_trigger, increment_scales, opening`

func scanGame(row interface{ Scan(...any) error }, g *Game) error {
	return row.Scan(&g.ID, &g.Date, &g.Size, &g.PlayerWhite, &g.PlayerBlack, &g.Notation, &g.Result, &g.TimerTime, &g.TimerInc,
		&g.RatingWhite, &g.RatingBlack, &g.Unrated, &g.Tournament, &g.Komi, &g.Pieces, &g.Capstones,
		&g.RatingChangeWhite, &g.RatingChangeBlack, &g.ExtraTimeAmount, &g.ExtraTimeTrigger, &g.IncrementScales, &g.Opening)
}

// IntFilter matches an integer column. Min and Max are inclusive bounds; In,
// if set, lists the allowed values. The zero value matches everything.
type IntFilter struct {
	Min, Max *int64
	In       []int64
}

// Search selects and orders games for the history listing.
type Search struct {
	ID, Date IntFilter
	// White and Black match player names exactly, ignoring case.
	White, Black string
	// Result is a result ("R-0", "1/2-1/2", ...) or "X-0" / "0-X" for any
	// white / black win.
	Result string
	// Mirror also matches the games with the colors swapped: the players
	// and the result are matched from the other side.
	Mirror bool
	// Equal requires these columns to have these values. Keys must be from
	// EqualColumns.
	Equal map[string]int64
	// Sort is a column from SortColumns.
	Sort          string
	Desc          bool
	Limit, Offset int
}

// EqualColumns are the integer columns Search.Equal may filter on.
var EqualColumns = map[string]bool{
	"size": true, "timertime": true, "timerinc": true, "komi": true, "unrated": true, "tournament": true,
	"extra_time_amount": true, "extra_time_trigger": true, "increment_scales": true,
}

// SortColumns are the columns the history can be sorted by.
var SortColumns = map[string]bool{}

func init() {
	for c := range strings.SplitSeq(gameColumns, ",") {
		SortColumns[strings.TrimSpace(c)] = true
	}
}

var (
	whiteWins = []string{"R-0", "F-0", "1-0"}
	blackWins = []string{"0-R", "0-F", "0-1"}
	flipped   = map[string]string{
		"R-0": "0-R", "F-0": "0-F", "1-0": "0-1", "0-R": "R-0", "0-F": "F-0", "0-1": "1-0", "X-0": "0-X", "0-X": "X-0",
	}
)

func results(r string) []string {
	switch r {
	case "":
		return nil
	case "X-0":
		return whiteWins
	case "0-X":
		return blackWins
	}
	return []string{r}
}

// History returns one page of the games s selects and the total number of
// games it selects.
func (s *Store) History(ctx context.Context, q Search) ([]Game, int, error) {
	if !SortColumns[q.Sort] {
		return nil, 0, fmt.Errorf("games history: cannot sort by %q", q.Sort)
	}
	for col := range q.Equal {
		if !EqualColumns[col] {
			return nil, 0, fmt.Errorf("games history: cannot filter on %q", col)
		}
	}
	order := "ASC"
	if q.Desc {
		order = "DESC"
	}
	countSQL, pageSQL, countArgs, pageArgs := q.queries()
	pageSQL += ` ORDER BY ` + q.Sort + ` ` + order + ` LIMIT ? OFFSET ?`
	pageArgs = append(pageArgs, q.Limit, q.Offset)

	var total int
	if err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count games: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, pageSQL, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()
	games := []Game{}
	for rows.Next() {
		var g Game
		if err := scanGame(rows, &g); err != nil {
			return nil, 0, fmt.Errorf("list games: %w", err)
		}
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list games: %w", err)
	}
	return games, total, nil
}

// condition is a SQL boolean expression and its arguments.
type condition struct {
	sql  []string // ANDed
	args []any
}

func (c *condition) add(sql string, args ...any) {
	c.sql = append(c.sql, sql)
	c.args = append(c.args, args...)
}

func (c condition) and(other condition) condition {
	return condition{append(slices.Clone(c.sql), other.sql...), append(slices.Clone(c.args), other.args...)}
}

func (c condition) where() string {
	if len(c.sql) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(c.sql, " AND ")
}

// queries returns the count query and the page query (without ORDER BY and
// LIMIT) for q.
//
// A mirrored search is two queries, one per color, rather than an OR: with
// bound parameters SQLite won't plan an OR across the two player indexes and
// scans the whole table instead. As a UNION ALL the page comes off both
// indexes already in id order, and the count reads only the indexes.
func (q Search) queries() (countSQL, pageSQL string, countArgs, pageArgs []any) {
	var shared condition
	intFilter := func(col string, f IntFilter) {
		if f.Min != nil {
			shared.add(col+" >= ?", *f.Min)
		}
		if f.Max != nil {
			shared.add(col+" <= ?", *f.Max)
		}
		if len(f.In) > 0 {
			args := make([]any, len(f.In))
			for i, v := range f.In {
				args[i] = v
			}
			shared.add(col+" IN ("+placeholders(len(f.In))+")", args...)
		}
	}
	intFilter("id", q.ID)
	intFilter("date", q.Date)
	for _, col := range slices.Sorted(maps.Keys(q.Equal)) {
		shared.add(col+" = ?", q.Equal[col])
	}

	// side matches the players and result from one color's point of view.
	side := func(white, black string, results []string) condition {
		var c condition
		if white != "" {
			c.add("player_white = ? COLLATE NOCASE", white)
		}
		if black != "" {
			c.add("player_black = ? COLLATE NOCASE", black)
		}
		if len(results) > 0 {
			args := make([]any, len(results))
			for i, r := range results {
				args[i] = r
			}
			c.add("result IN ("+placeholders(len(results))+")", args...)
		}
		return c
	}
	direct := side(q.White, q.Black, results(q.Result))

	if !q.Mirror || len(direct.sql) == 0 {
		all := shared.and(direct)
		return `SELECT count(*) FROM games` + all.where(), `SELECT ` + gameColumns + ` FROM games` + all.where(), all.args, all.args
	}

	mirrorResult, ok := flipped[q.Result]
	if !ok {
		mirrorResult = q.Result // draws and unknown results read the same both ways
	}
	first := shared.and(direct)
	second := shared.and(side(q.Black, q.White, results(mirrorResult)))
	// UNION drops games both sides match (self-games) by id; the page's
	// UNION ALL excludes them from the second half instead.
	countSQL = `SELECT count(*) FROM (SELECT id FROM games` + first.where() + ` UNION SELECT id FROM games` + second.where() + `)`
	countArgs = append(slices.Clone(first.args), second.args...)
	notFirst := second.and(condition{[]string{"NOT (" + strings.Join(direct.sql, " AND ") + ")"}, direct.args})
	pageSQL = `SELECT ` + gameColumns + ` FROM games` + first.where() + ` UNION ALL SELECT ` + gameColumns + ` FROM games` + notFirst.where()
	pageArgs = append(slices.Clone(first.args), notFirst.args...)
	return countSQL, pageSQL, countArgs, pageArgs
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// ByID returns the game with this ID, or ErrNotFound.
func (s *Store) ByID(ctx context.Context, id int) (Game, error) {
	var g Game
	err := scanGame(s.db.QueryRowContext(ctx, `SELECT `+gameColumns+` FROM games WHERE id = ?`, id), &g)
	if errors.Is(err, sql.ErrNoRows) {
		return Game{}, ErrNotFound
	}
	if err != nil {
		return Game{}, fmt.Errorf("get game %d: %w", id, err)
	}
	return g, nil
}
