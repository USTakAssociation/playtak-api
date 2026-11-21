package rating

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
)

// Files are the job's files outside the databases.
type Files struct {
	// Cursor holds "<last rated game id> hash", the format the Node job
	// used, so either job can take over from the other.
	Cursor string
	// RatingList is where the gzipped JSON list for the ratings page is
	// written. Empty skips it.
	RatingList string
}

// Result summarizes a run.
type Result struct {
	From, To int // cursor before and after
	Games    int // games rows written
}

// writeBusyWait bounds how long Run waits for the game server to release the
// write lock.
const writeBusyWait = 30 * time.Second

// Run rates the games after the cursor and writes the results. conn must be
// the games database with the players database attached as "p" (see
// AttachPlayers). The games and players updates commit together; the cursor
// and rating list are written only after that commit succeeds.
func Run(ctx context.Context, conn *sql.Conn, files Files, now time.Time) (Result, error) {
	cursor, err := readCursor(files.Cursor)
	if err != nil {
		return Result{}, err
	}
	players, err := loadPlayers(ctx, conn)
	if err != nil {
		return Result{}, err
	}
	games, err := loadGames(ctx, conn, cursor)
	if err != nil {
		return Result{}, err
	}

	byName := make(map[string]*Player, len(players))
	for _, p := range players {
		if _, dup := byName[p.Name]; !dup { // Node matched the first player with the name
			byName[p.Name] = p
		}
	}
	updates, newCursor := rateGames(byName, games, cursor, now.UnixMilli())

	nowMs := now.UnixMilli()
	participation := make([]float64, len(players))
	for i, p := range players {
		participation[i] = adjustedRating(p, nowMs)
	}

	if err := writeAll(ctx, conn, updates, players, participation); err != nil {
		return Result{}, err
	}
	// shortcut: a crash between the commit above and this write makes the
	// next run rate these games again. Moving the cursor into the games DB
	// closes that gap once the Node job (which reads this file) is retired.
	if err := writeFileAtomic(files.Cursor, []byte(strconv.Itoa(newCursor)+" hash")); err != nil {
		return Result{}, fmt.Errorf("write rating cursor: %w", err)
	}
	if files.RatingList != "" {
		if err := writeRatingList(files.RatingList, players, participation); err != nil {
			return Result{}, err
		}
	}
	return Result{From: cursor, To: newCursor, Games: len(updates)}, nil
}

// AttachPlayers attaches the players database to conn as "p".
func AttachPlayers(ctx context.Context, conn *sql.Conn, path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("players database: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS p`, path); err != nil {
		return fmt.Errorf("attach players database: %w", err)
	}
	return nil
}

func readCursor(path string) (int, error) {
	// A missing cursor is an error, not "start from 0": restarting from 0
	// would rate every game a second time.
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read rating cursor: %w", err)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(raw)), " ")
	cursor, err := strconv.Atoi(first)
	if err != nil {
		return 0, fmt.Errorf("rating cursor %s: %q is not a game id", path, first)
	}
	return cursor, nil
}

func loadPlayers(ctx context.Context, conn *sql.Conn) ([]*Player, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, COALESCE(name, ''), COALESCE(unrated, 0), COALESCE(isbot, 0), COALESCE(rating, 0), COALESCE(boost, 0),
		       COALESCE(ratedgames, 0), COALESCE(maxrating, 0), COALESCE(ratingage, 0), COALESCE(fatigue, '{}')
		FROM p.players`)
	if err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}
	defer rows.Close()
	var players []*Player
	for rows.Next() {
		p := &Player{}
		var fatigue string
		if err := rows.Scan(&p.ID, &p.Name, &p.Unrated, &p.IsBot, &p.Rating, &p.Boost,
			&p.RatedGames, &p.MaxRating, &p.RatingAge, &fatigue); err != nil {
			return nil, fmt.Errorf("load players: %w", err)
		}
		p.Fatigue = parseFatigue(fatigue)
		if p.Unrated {
			p.Rating, p.MaxRating = 0, 0
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}
	return players, nil
}

func loadGames(ctx context.Context, conn *sql.Conn, cursor int) ([]Game, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, date, player_white, player_black, result, unrated, size, timertime, timerinc, pieces, capstones, length(notation)
		FROM games WHERE id > ? AND date > ? ORDER BY id LIMIT ?`, cursor, firstNamedGame, batchSize)
	if err != nil {
		return nil, fmt.Errorf("load games: %w", err)
	}
	defer rows.Close()
	var games []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.Date, &g.White, &g.Black, &g.Result, &g.Unrated, &g.Size,
			&g.TimerTime, &g.TimerInc, &g.Pieces, &g.Capstones, &g.NotationLength); err != nil {
			return nil, fmt.Errorf("load games: %w", err)
		}
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load games: %w", err)
	}
	return games, nil
}

func writeAll(ctx context.Context, conn *sql.Conn, updates []GameUpdate, players []*Player, participation []float64) error {
	// IMMEDIATE takes the write locks on both files up front, so the game
	// server's writes wait (busy_timeout) rather than deadlocking with ours.
	err := db.RetryBusy(ctx, writeBusyWait, func() error {
		_, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`)
		return err
	})
	if err != nil {
		return fmt.Errorf("begin rating transaction: %w", err)
	}
	if err := writeRows(ctx, conn, updates, players, participation); err != nil {
		conn.ExecContext(context.WithoutCancel(ctx), `ROLLBACK`)
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		conn.ExecContext(context.WithoutCancel(ctx), `ROLLBACK`)
		return fmt.Errorf("commit ratings: %w", err)
	}
	return nil
}

func writeRows(ctx context.Context, conn *sql.Conn, updates []GameUpdate, players []*Player, participation []float64) error {
	gameStmt, err := conn.PrepareContext(ctx, `
		UPDATE games SET rating_white = ?, rating_black = ?, rating_change_white = ?, rating_change_black = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare game update: %w", err)
	}
	defer gameStmt.Close()
	for _, u := range updates {
		if _, err := gameStmt.ExecContext(ctx, u.RatingWhite, u.RatingBlack, u.ChangeWhite, u.ChangeBlack, u.ID); err != nil {
			return fmt.Errorf("update game %d: %w", u.ID, err)
		}
	}

	playerStmt, err := conn.PrepareContext(ctx, `
		UPDATE p.players SET rating = ?, boost = ?, ratedgames = ?, maxrating = ?, ratingage = ?, fatigue = ?, participation_rating = ?
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare player update: %w", err)
	}
	defer playerStmt.Close()
	participationStmt, err := conn.PrepareContext(ctx, `UPDATE p.players SET participation_rating = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare participation update: %w", err)
	}
	defer participationStmt.Close()
	for i, p := range players {
		if !p.changed {
			if _, err := participationStmt.ExecContext(ctx, participation[i], p.ID); err != nil {
				return fmt.Errorf("update player %d: %w", p.ID, err)
			}
			continue
		}
		// Stored as plain JSON; Node's parseFatigue reads it too.
		fatigue, err := json.Marshal(p.Fatigue)
		if err != nil {
			return fmt.Errorf("encode fatigue for player %d: %w", p.ID, err)
		}
		if _, err := playerStmt.ExecContext(ctx, p.Rating, p.Boost, p.RatedGames, p.MaxRating, p.RatingAge,
			string(fatigue), participation[i], p.ID); err != nil {
			return fmt.Errorf("update player %d: %w", p.ID, err)
		}
	}
	return nil
}

// writeRatingList writes the ratings page's list: [name, participation
// rating, rating, rated games, isbot] per rated player (and unrated bots),
// highest participation rating first.
func writeRatingList(path string, players []*Player, participation []float64) error {
	type entry struct {
		row           [5]any
		participation float64
	}
	var list []entry
	for i, p := range players {
		if p.RatedGames > 0 || (p.Unrated && p.IsBot) {
			isBot := 0
			if p.IsBot {
				isBot = 1
			}
			list = append(list, entry{
				row:           [5]any{p.Name, int(math.Floor(participation[i])), int(math.Floor(p.Rating)), p.RatedGames, isBot},
				participation: participation[i],
			})
		}
	}
	// Sorted on the unfloored value, stably, like Node.
	slices.SortStableFunc(list, func(a, b entry) int { return cmp.Compare(b.participation, a.participation) })
	rows := make([][5]any, len(list))
	for i, e := range list {
		rows[i] = e.row
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("encode rating list: %w", err)
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write(raw)
	if err := zw.Close(); err != nil {
		return fmt.Errorf("compress rating list: %w", err)
	}
	if err := writeFileAtomic(path, buf.Bytes()); err != nil {
		return fmt.Errorf("write rating list: %w", err)
	}
	return nil
}

// writeFileAtomic replaces path with data so readers never see a partial
// file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
