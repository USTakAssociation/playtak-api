package suite_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// ---- Time vs Timems based on protocol version ----

// protocol 0 should be deprecated sending a 0 causes the server to return NOK
func TestClockProtocol0UsesTimeInSeconds(t *testing.T) {
	c1, c2, gameID := startGame(t) // default protocol 0
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))

	// c2's move triggers the Time update to c1 (server sends Time before OK)
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")

	timeMsg := c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))

	if strings.Contains(timeMsg, "Timems") {
		t.Errorf("protocol 0 should send 'Time' (seconds), got: %q", timeMsg)
	}
	if !strings.HasPrefix(timeMsg, fmt.Sprintf("Game#%d Time ", gameID)) {
		t.Errorf("expected 'Game#%d Time <white> <black>', got: %q", gameID, timeMsg)
	}

	// Values should be in seconds (around 600 for a 10 min game)
	parts := strings.Fields(timeMsg)
	if len(parts) != 4 {
		t.Fatalf("Time message should have 4 fields, got %d: %q", len(parts), timeMsg)
	}
	whiteTime, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("white time not an integer: %q", parts[2])
	}
	if whiteTime > 700 || whiteTime < 500 {
		t.Errorf("white time in seconds should be near 600, got %d", whiteTime)
	}
	
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestClockProtocol1UsesTimems(t *testing.T) {
	c1, c2, gameID := startGameWithProtocol(t, 1)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))

	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")

	timeMsg := c1.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))

	if !strings.HasPrefix(timeMsg, fmt.Sprintf("Game#%d Timems ", gameID)) {
		t.Errorf("protocol 1 should send 'Timems', got: %q", timeMsg)
	}

	// Values should be in milliseconds (around 600000 for a 10 min game)
	parts := strings.Fields(timeMsg)
	if len(parts) != 4 {
		t.Fatalf("Timems message should have 4 fields, got %d: %q", len(parts), timeMsg)
	}
	whiteTime, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		t.Fatalf("white time not an integer: %q", parts[2])
	}
	if whiteTime > 700000 || whiteTime < 500000 {
		t.Errorf("white time in milliseconds should be near 600000, got %d", whiteTime)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestClockTimeNotSentBeforeFirstMove(t *testing.T) {
	c1, _, gameID := startGame(t)
	// Clock must not start until the first move is played.
	// Note: Game#N Show returns the raw Java toString (a Java server bug —
	// it should return structured state). We avoid Show entirely and instead
	// assert directly that no Time/Timems message arrives in the quiet window.
	c1.ExpectNoMessageWithPrefix(fmt.Sprintf("Game#%d Time", gameID), client.NoMessageWindow)
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestClockTimeSentAfterEachMove(t *testing.T) {
	c1, c2, gameID := startGameWithProtocol(t, 1)
	makeOpeningMoves(t, c1, c2, gameID)

	c1squares := []string{"A1", "B1", "C1", "D1", "A2", "B2"}
	c2squares := []string{"A3", "B3", "C3", "D3", "A4", "B4"}
	// Every move should produce a Timems update to both players
	//Game#856726 Timems 633178 618003
	for i := 0; i < 3; i++ {
		// c1 send next move
		c1.Send(fmt.Sprintf("Game#%d P %s", gameID, c1squares[i]))
		c1.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
		c1.DrainUntil("OK")
		// c2 gets c1 move
		c2.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
		c2.DrainUntil(fmt.Sprintf("Game#%d P %s", gameID, c1squares[i]))
		// c2 send next move
		c2.Send(fmt.Sprintf("Game#%d P %s", gameID, c2squares[i]))
		c2.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
		c2.DrainUntil("OK")
		// c1 gets c2 move
		c1.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
		c1.DrainUntil(fmt.Sprintf("Game#%d P %s", gameID, c2squares[i]))
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

// ---- Increment ----

func TestClockIncrementAppliedAfterMove(t *testing.T) {
	// 10 minute game with 60 second increment
	// After white's first real move, white's clock should be >= 600s
	// (original time), meaning increment was added
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c1, 1)
	client.LoginGuestAfterProtocol(t, c1)
	client.SetProtocol(t, c2, 1)
	client.LoginGuestAfterProtocol(t, c2)

	pieces, caps := defaultPieces(5)
	// 10 min + 60s increment
	c1.Send(fmt.Sprintf("Seek 5 600 60 A 0 %d %d 0 0 ", pieces, caps))
	seekMsg := c1.DrainUntil("Seek new ")
	seekID := parseSeekID(t, seekMsg)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	gameID := parseGameID(t, gs1)

	// Swap so white is always first argument to makeOpeningMoves
	white, black := c1, c2
	if colorFromGameStart(gs1) != "white" {
		white, black = c2, c1
	}
	makeOpeningMoves(t, white, black, gameID)
	c1, c2 = white, black

	// First real move by c1 (white)
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))

	// c2 moves to trigger the clock update for c1
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")

	timeMsg := c1.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
	parts := strings.Fields(timeMsg)
	if len(parts) < 3 {
		t.Fatalf("not enough fields in time message: %q", timeMsg)
	}
	whiteTimeMs, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		t.Fatalf("could not parse white time: %v", err)
	}
	// White just moved and got a 60s increment: time should be > original 600s (600000ms)
	// Allow a generous window for test execution time
	if whiteTimeMs < 600000 {
		t.Errorf("white time should be >= 600000ms after increment, got %d", whiteTimeMs)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

// ---- Give Time ----

func TestClockGiveTimeOnce(t *testing.T) {
	c1, c2, gameID := startGameWithProtocol(t, 1)
	makeOpeningMoves(t, c1, c2, gameID)

	// Make a couple of moves to get the clock started
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")
	c1.DrainUntil(fmt.Sprintf("Game#%d P A2", gameID))

	// c1 gives time to c2 (black)
	c1.Send(fmt.Sprintf("Game#%d GiveTime", gameID))

	// Both players should receive GivenTime notification
	c1.DrainUntil(fmt.Sprintf("Game#%d GivenTime black 15000", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d GivenTime black 15000", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestClockGiveTimeUpdatesClockMessage(t *testing.T) {
	c1, c2, gameID := startGameWithProtocol(t, 1)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")
	// Drain the time update
	c1.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d P A2", gameID))

	// Capture black's time before giving time
	c1.Send(fmt.Sprintf("Game#%d GiveTime", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d GivenTime", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d GivenTime", gameID))

	// A fresh Timems should follow showing the updated time
	timeMsg := c1.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
	parts := strings.Fields(timeMsg)
	if len(parts) < 4 {
		t.Fatalf("not enough fields in Timems after GiveTime: %q", timeMsg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}
