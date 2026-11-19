// Package dbtest creates throwaway databases for tests.
package dbtest

import (
	"context"
	"database/sql"
	_ "embed"
	"path/filepath"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
)

//go:embed players.sql
var playersSchema string

//go:embed games.sql
var gamesSchema string

// NewPlayersDB returns an empty players database, with the post-migration
// schema, that is deleted when the test ends.
func NewPlayersDB(t testing.TB) *sql.DB {
	t.Helper()
	return newDB(t, "players.db", playersSchema)
}

// NewGamesDB returns an empty games database that is deleted when the test
// ends.
func NewGamesDB(t testing.TB) *sql.DB {
	t.Helper()
	return newDB(t, "games.db", gamesSchema)
}

func newDB(t testing.TB, name, schema string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	// db.Open refuses to create files, so create it directly first.
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(schema); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	conn, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
