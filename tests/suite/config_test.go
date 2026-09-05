package suite_test

import (
	"os"
	"testing"
)

// telnetAddr returns the telnet address of the server under test.
// Defaults to localhost:10000 if TAK_TELNET_ADDR is not set.
func telnetAddr(t *testing.T) string {
	t.Helper()
	if addr := os.Getenv("TAK_TELNET_ADDR"); addr != "" {
		return addr
	}
	return "localhost:10000"
}

// wsAddr returns the WebSocket address of the server under test.
// Defaults to localhost:9999 if TAK_WS_ADDR is not set.
func wsAddr(t *testing.T) string {
	t.Helper()
	if addr := os.Getenv("TAK_WS_ADDR"); addr != "" {
		return addr
	}
	return "localhost:9999"
}

// testUser returns credentials for a pre-seeded test account.
// Accounts must be created with scripts/development/add_user.sh before running tests.
// All accounts use the password "password".
// The test is skipped automatically if TAK_SKIP_AUTH_TESTS=1.
func testUser(t *testing.T, n int) (username, password string) {
	t.Helper()
	if os.Getenv("TAK_SKIP_AUTH_TESTS") == "1" {
		t.Skip("skipping: TAK_SKIP_AUTH_TESTS is set")
	}
	switch n {
	case 1:
		return "testuser1", "password"
	case 2:
		return "testuser2", "password"
	case 3:
		return "testuser3", "password"
	default:
		t.Fatalf("unknown test user number %d (valid: 1-3)", n)
	}
	return "", ""
}

// adminUser returns credentials for an admin account.
// The test is skipped if TAK_ADMIN_USER or TAK_ADMIN_PASS are not set.
func adminUser(t *testing.T) (username, password string) {
	t.Helper()
	u := os.Getenv("TAK_ADMIN_USER")
	p := os.Getenv("TAK_ADMIN_PASS")
	if u == "" || p == "" {
		t.Skip("skipping: TAK_ADMIN_USER and TAK_ADMIN_PASS must be set for admin tests")
	}
	return u, p
}
