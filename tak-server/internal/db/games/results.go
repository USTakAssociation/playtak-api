package games

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Finisher records a finished game.
type Finisher interface {
	Finish(ctx context.Context, id int, notation, result string) error
}

// Result is a finished game waiting to be written to the database.
type Result struct {
	ID       int    `json:"id"`
	Notation string `json:"notation"`
	Result   string `json:"result"`
}

const (
	minRetry = 5 * time.Second
	maxRetry = 5 * time.Minute
)

// Results writes finished games to the database without ever losing one.
// Save first appends the result to a file and returns; a background loop
// writes pending results to the database and removes them from the file,
// retrying with backoff while the database is unavailable. Results still in
// the file when the server stops are written on the next start.
type Results struct {
	store Finisher
	path  string
	log   *slog.Logger

	mu   sync.Mutex // guards the file
	wake chan struct{}
}

func NewResults(store Finisher, path string, log *slog.Logger) *Results {
	return &Results{store: store, path: path, log: log, wake: make(chan struct{}, 1)}
}

// Save durably records r for writing.
func (r *Results) Save(res Result) error {
	line, err := json.Marshal(res)
	if err != nil {
		return err
	}
	r.mu.Lock()
	err = appendSync(r.path, append(line, '\n'))
	r.mu.Unlock()
	if err != nil {
		return fmt.Errorf("save result of game %d: %w", res.ID, err)
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
	return nil
}

// Run writes pending results until ctx is cancelled, starting with any left
// from a previous run.
func (r *Results) Run(ctx context.Context) {
	retry := minRetry
	for {
		remaining := r.flush(ctx)
		var timer <-chan time.Time
		if remaining > 0 {
			r.log.Warn("game results not yet saved, will retry", "pending", remaining, "retry_in", retry)
			timer = time.After(retry)
			retry = min(retry*2, maxRetry)
		} else {
			retry = minRetry
		}
		select {
		case <-ctx.Done():
			if remaining > 0 {
				r.log.Warn("stopping with unsaved game results; they will be saved on the next start",
					"pending", remaining, "file", r.path)
			}
			return
		case <-r.wake:
		case <-timer:
		}
	}
}

// flush writes every pending result it can and returns how many are left.
func (r *Results) flush(ctx context.Context) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending, err := r.read()
	if err != nil {
		r.log.Error("read pending game results", "file", r.path, "err", err)
		return 1
	}
	if len(pending) == 0 {
		return 0
	}
	var left []Result
	for _, p := range pending {
		wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := r.store.Finish(wctx, p.ID, p.Notation, p.Result)
		cancel()
		if err != nil {
			r.log.Error("save game result", "game", p.ID, "err", err)
			left = append(left, p)
		}
	}
	if err := r.write(left); err != nil {
		r.log.Error("rewrite pending game results", "file", r.path, "err", err)
	}
	return len(left)
}

func (r *Results) read() ([]Result, error) {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Result
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, 16<<20) // notation for very long games
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var res Result
		if err := json.Unmarshal(sc.Bytes(), &res); err != nil {
			// A line cut short by a crash mid-write; nothing to recover.
			r.log.Error("skipping unreadable pending game result", "line", sc.Text(), "err", err)
			continue
		}
		out = append(out, res)
	}
	return out, sc.Err()
}

// write replaces the file with results, removing it if there are none.
func (r *Results) write(results []Result) error {
	if len(results) == 0 {
		err := os.Remove(r.path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var buf bytes.Buffer
	for _, res := range results {
		line, _ := json.Marshal(res)
		buf.Write(line)
		buf.WriteByte('\n')
	}
	tmp := r.path + ".tmp"
	if err := writeSync(tmp, buf.Bytes()); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func appendSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeSync(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
