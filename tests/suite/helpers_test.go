package suite_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// newObserver creates a fresh guest client that is observing the given game.
// Returns after the Observe confirmation has been received.
func newObserver(t *testing.T, gameID int) *client.Client {
	t.Helper()
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.Send(fmt.Sprintf("Observe %d", gameID))
	c.DrainUntil(fmt.Sprintf("Observe %d ", gameID))
	return c
}

// startGame creates two fresh guest clients, posts a 5x5 10min+30s seek,
// has the second client accept it, and returns both clients plus the game ID.
// The first returned client is always WHITE (moves first in the opening).
// Both clients have consumed all setup messages (seek lists, game start, etc).
func startGame(t *testing.T) (white, black *client.Client, gameID int) {
	t.Helper()
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)
	seekID := postSeek(t, c1, 5, 600, 30)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")

	gameID = parseGameID(t, gs1)
	if colorFromGameStart(gs1) == "white" {
		return c1, c2, gameID
	}
	return c2, c1, gameID
}

// startGameWithProtocol is like startGame but sets a protocol version on both clients.
// The first returned client is always WHITE (moves first in the opening).
func startGameWithProtocol(t *testing.T, version int) (white, black *client.Client, gameID int) {
	t.Helper()
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c1, version)
	client.LoginGuestAfterProtocol(t, c1)
	client.SetProtocol(t, c2, version)
	client.LoginGuestAfterProtocol(t, c2)

	seekID := postSeek(t, c1, 5, 600, 30)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")

	gameID = parseGameID(t, gs1)
	if colorFromGameStart(gs1) == "white" {
		return c1, c2, gameID
	}
	return c2, c1, gameID
}

// postSeek posts a V2 seek (size time incr A komi pieces capstones unrated tournament opponent)
// and returns the seek ID parsed from the "Seek new <id> ..." broadcast.
func postSeek(t *testing.T, c *client.Client, size, timeSecs, incr int) int {
	t.Helper()
	pieces, caps := defaultPieces(size)
	// V2 format: Seek <size> <time> <incr> <color> <komi> <pieces> <capstones> <unrated> <tournament> <opponent>
	c.Send(fmt.Sprintf("Seek %d %d %d W 0 %d %d 0 0 ", size, timeSecs, incr, pieces, caps))
	msg := c.DrainUntil("Seek new ")
	return parseSeekID(t, msg)
}

// makeOpeningMoves plays the mandatory swap opening for a 5x5 game.
// white and black MUST be the correct players (white moves first).
// Use startGame / startGameWithProtocol which guarantee first return = white.
//
// Uses E1 (white) and A5 (black) as opening squares, leaving the rest of
// the board (including A1-D5) free for subsequent test moves.
// Both clients drain Time/Timems before OK so the caller starts clean.
func makeOpeningMoves(t *testing.T, white, black *client.Client, gameID int) {
	t.Helper()
	white.Send(fmt.Sprintf("Game#%d P E1", gameID))
	white.DrainUntil("OK")
	black.DrainUntil(fmt.Sprintf("Game#%d P E1", gameID))

	black.Send(fmt.Sprintf("Game#%d P A5", gameID))
	black.DrainUntil("OK")
	white.DrainUntil(fmt.Sprintf("Game#%d P A5", gameID))
}

// parseSeekID extracts the integer seek ID from a "Seek new <id> ..." message.
func parseSeekID(t *testing.T, msg string) int {
	t.Helper()
	trimmed := strings.TrimPrefix(msg, "Seek new ")
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		t.Fatalf("helpers: could not parse seek ID from %q", msg)
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("helpers: seek ID is not an integer in %q: %v", msg, err)
	}
	return id
}

// parseGameID extracts the integer game ID from a "Game Start <id> ..." message.
func parseGameID(t *testing.T, msg string) int {
	t.Helper()
	trimmed := strings.TrimPrefix(msg, "Game Start ")
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		t.Fatalf("helpers: could not parse game ID from %q", msg)
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("helpers: game ID is not an integer in %q: %v", msg, err)
	}
	return id
}

// parse rematch seek ID from Accept Rematch <id> message
func parseRematchSeekID(t *testing.T, msg string) int {
	t.Helper()
	trimmed := strings.TrimPrefix(msg, "Accept Rematch ")
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		t.Fatalf("helpers: could not parse rematch seek ID from %q", msg)
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("helpers: rematch seek ID is not an integer in %q: %v", msg, err)
	}
	return id
}
// colorFromGameStart parses the YOUR_color field from a "Game Start ..." message.
// Works for both protocol 0/1 and 2+ formats by finding "vs" and reading 2 fields ahead.
//
//	Protocol 0/1: Game Start <id> <size> <white> vs <black> <YOUR_color> ...
//	Protocol 2+:  Game Start <id> <white> vs <black> <YOUR_color> <size> ...
func colorFromGameStart(msg string) string {
	parts := strings.Fields(msg)
	for i, p := range parts {
		if p == "vs" && i+2 < len(parts) {
			return parts[i+2]
		}
	}
	return ""
}

// defaultPieces returns the standard piece counts for a given board size,
// matching the values used by the Java server for V1 seeks.
func defaultPieces(size int) (pieces, capstones int) {
	switch size {
	case 3:
		return 10, 0
	case 4:
		return 15, 0
	case 5:
		return 21, 1
	case 6:
		return 30, 1
	case 7:
		return 40, 2
	case 8:
		return 50, 2
	default:
		return 21, 1
	}
}
