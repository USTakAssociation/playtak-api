// Package db opens the server's SQLite databases.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path and verifies it is reachable. It does
// not create the file: a missing database is a deployment error.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	// The filename is read as a URI, so characters such as %, ? and # in the
	// path must be escaped. Transactions take the write lock up front
	// (_txlock): in WAL mode, which prod uses, a transaction that reads and
	// then writes fails at once if another connection wrote in between, and
	// busy_timeout doesn't help. Every transaction here writes anyway.
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() + "?mode=rw&" + url.Values{
		"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"},
		"_txlock": {"immediate"},
	}.Encode()
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return conn, nil
}

// WithTransaction runs fn inside a transaction on conn. The transaction is
// committed if fn returns nil and rolled back otherwise.
func WithTransaction(ctx context.Context, conn *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// IsBusy reports whether err means another connection holds a lock on the
// database (SQLITE_BUSY or SQLITE_LOCKED), so the operation may succeed if
// retried.
func IsBusy(err error) bool {
	var se interface{ Code() int }
	if !errors.As(err, &se) {
		return false
	}
	code := se.Code() & 0xff // primary result code
	return code == 5 || code == 6
}

// RetryBusy runs fn until it succeeds, fails with a non-busy error, or
// maxWait passes, backing off between attempts.
func RetryBusy(ctx context.Context, maxWait time.Duration, fn func() error) error {
	deadline := time.Now().Add(maxWait)
	wait := 50 * time.Millisecond
	for {
		err := fn()
		if err == nil || !IsBusy(err) || time.Now().Add(wait).After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
		wait = min(wait*2, time.Second)
	}
}
