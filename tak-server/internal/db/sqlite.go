// Package db opens the server's SQLite databases.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path and verifies it is reachable. It does
// not create the file: a missing database is a deployment error.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := "file:" + path + "?mode=rw&" + url.Values{
		"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"},
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
