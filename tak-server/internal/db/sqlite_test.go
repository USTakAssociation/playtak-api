package db

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// lockedDB returns a database path, a connection that won't wait for locks,
// and a function that holds the write lock from another connection until
// the returned release is called.
func lockedDB(t *testing.T) (*sql.DB, func() (release func())) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.db")
	setup, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(`CREATE TABLE t (v INT)`); err != nil {
		t.Fatal(err)
	}
	setup.Close()

	impatient, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { impatient.Close() })

	lock := func() func() {
		holder, err := sql.Open("sqlite", "file:"+path)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := holder.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
			t.Fatal(err)
		}
		return func() {
			conn.ExecContext(context.Background(), `COMMIT`)
			conn.Close()
			holder.Close()
		}
	}
	return impatient, lock
}

func TestIsBusy(t *testing.T) {
	conn, lock := lockedDB(t)
	release := lock()
	defer release()
	_, err := conn.Exec(`INSERT INTO t VALUES (1)`)
	if err == nil {
		t.Fatal("expected a busy error while another connection holds the lock")
	}
	if !IsBusy(err) {
		t.Errorf("IsBusy(%v) = false", err)
	}
	if IsBusy(errors.New("disk on fire")) || IsBusy(nil) {
		t.Error("non-SQLite errors aren't busy")
	}
}

func TestRetryBusyWaitsForLock(t *testing.T) {
	conn, lock := lockedDB(t)
	release := lock()
	time.AfterFunc(300*time.Millisecond, release)

	start := time.Now()
	err := RetryBusy(context.Background(), 5*time.Second, func() error {
		_, err := conn.Exec(`INSERT INTO t VALUES (1)`)
		return err
	})
	if err != nil {
		t.Fatalf("RetryBusy: %v", err)
	}
	if time.Since(start) < 250*time.Millisecond {
		t.Error("should have waited for the lock")
	}
}

func TestRetryBusyGivesUp(t *testing.T) {
	conn, lock := lockedDB(t)
	release := lock()
	defer release()
	err := RetryBusy(context.Background(), 200*time.Millisecond, func() error {
		_, err := conn.Exec(`INSERT INTO t VALUES (1)`)
		return err
	})
	if !IsBusy(err) {
		t.Errorf("err = %v, want a busy error after giving up", err)
	}
}

func TestRetryBusyDoesntRetryOtherErrors(t *testing.T) {
	calls := 0
	err := RetryBusy(context.Background(), time.Second, func() error {
		calls++
		return errors.New("syntax error")
	})
	if err == nil || calls != 1 {
		t.Errorf("err %v after %d calls, want one call", err, calls)
	}
}

func TestOpenPathWithURICharacters(t *testing.T) {
	// SQLite reads the filename as a URI, so these must not be decoded or
	// cut off: "%20" is not a space, and "?" and "#" are part of the name.
	for _, dirName := range []string{"tak%20data", "a?b", "c#d", "spaced dir"} {
		dir := filepath.Join(t.TempDir(), dirName)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "players.db")
		seed, err := sql.Open("sqlite", "file:"+(&url.URL{Path: path}).EscapedPath())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := seed.Exec(`CREATE TABLE marker (x)`); err != nil {
			t.Fatalf("%s: create: %v", dirName, err)
		}
		seed.Close()
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: seed went elsewhere: %v", dirName, err)
		}

		conn, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("%s: %v", dirName, err)
		}
		var n int
		err = conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'marker'`).Scan(&n)
		conn.Close()
		if err != nil || n != 1 {
			t.Errorf("%s: opened a different database (marker %d, %v)", dirName, n, err)
		}
	}
}
