package suite_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// ---- Game start ----

func TestGameStartSentToBothPlayers(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gameId := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, parseGameID(t, gameId))
}

func TestGameStartContainsCorrectSize(t *testing.T) {
	sizes := []int{5, 6}
	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("size%d", size), func(t *testing.T) {
			c1 := client.New(t, telnetAddr(t))
			c2 := client.New(t, telnetAddr(t))
			client.LoginGuest(t, c1)
			client.LoginGuest(t, c2)

			seekID := postSeek(t, c1, size, 600, 30)
			c2.Send(fmt.Sprintf("Accept %d", seekID))

			gs := c1.DrainUntil("Game Start ")
			parts := strings.Fields(gs)
			if len(parts) != 14 {
				t.Fatalf("protocol 0-1 Game Start should have 14 fields, got %d: %q", len(parts), gs)
			}
			if got, want := parts[3], strconv.Itoa(size); got != want {
				t.Errorf("Game Start board size at index 3 should be %q, got %q in %q", want, got, gs)
			}
			// clean up active games so the test doesn't leave a dangling game
			client.CleanupActiveGames(t, c1, parseGameID(t, gs))
		})
	}
}

func TestGameStartColorAssignment(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gs1 := c1.DrainUntil("Game Start ")
	gs2 := c2.DrainUntil("Game Start ")

	// Colors must be opposite
	c1IsWhite := strings.Contains(gs1, " white ")
	c2IsWhite := strings.Contains(gs2, " white ")

	if c1IsWhite == c2IsWhite {
		t.Errorf("both players got the same color\n  c1: %q\n  c2: %q", gs1, gs2)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, parseGameID(t, gs1))
}

// ---- Place moves ----

func TestGamePlaceFlatFirstMove(t *testing.T) {
	c1, c2, gameID := startGame(t)
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect(fmt.Sprintf("Game#%d P A1", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceMoveEchoedToOpponent(t *testing.T) {
	c1, c2, gameID := startGame(t)
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	got := c2.Recv()
	if got != fmt.Sprintf("Game#%d P A1", gameID) {
		t.Errorf("opponent should receive move echo, got %q", got)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceCapstoneOnFirstMoveFails(t *testing.T) {
	c1, _, gameID := startGame(t)
	// First move must be a flat stone — capstone is illegal
	c1.Send(fmt.Sprintf("Game#%d P A1 C", gameID))
	c1.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceWallOnFirstMoveFails(t *testing.T) {
	c1, _, gameID := startGame(t)
	c1.Send(fmt.Sprintf("Game#%d P A1 W", gameID))
	c1.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceMoveNotYourTurn(t *testing.T) {
	c1, c2, gameID := startGame(t)
	// c2 tries to move first — should be rejected
	c2.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c2.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceOnOccupiedSquareFails(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	// c1 places on A5
	c1.Send(fmt.Sprintf("Game#%d P A5", gameID))
	c1.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceCapstoneAfterOpening(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	// Now c1 can place a capstone
	c1.Send(fmt.Sprintf("Game#%d P B1 C", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGamePlaceWallAfterOpening(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P B1 W", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

// ---- Move moves (stack moves) ----

func TestGameMoveStackBasic(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	// Build a two-stone stack on A1 then move it
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))
	c1.Send(fmt.Sprintf("Game#%d P B1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d", gameID))
	c2.Send(fmt.Sprintf("Game#%d P B2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	// Move top of A1 to B1 (1 stone, 1 square east, drop 1)
	c1.Send(fmt.Sprintf("Game#%d M A1 B1 1", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d M A1 B1 1", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGameMoveStackOutOfBoundsFails(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d P", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID));
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d P", gameID))

	// Try to move off the board (A1 moving west to before A)
	c1.Send(fmt.Sprintf("Game#%d M A1 A0 1", gameID))
	c1.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGameMoveFirstMoveFails(t *testing.T) {
	c1, _, gameID := startGame(t)
	// First move must be a placement, not a stack move
	c1.Send(fmt.Sprintf("Game#%d M A1 B1 1", gameID))
	c1.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

// ---- Win conditions ----

func TestGameRoadWinWhite(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	// Build a simple road for c1 (white): A1-A2-A3-A4-A5
	// Each move: c1 places, c2 responds somewhere irrelevant
	moves := [][2]string{
		{fmt.Sprintf("Game#%d P A1", gameID), fmt.Sprintf("Game#%d P B2", gameID)},
		{fmt.Sprintf("Game#%d P A2", gameID), fmt.Sprintf("Game#%d P B3", gameID)},
		{fmt.Sprintf("Game#%d P A3", gameID), fmt.Sprintf("Game#%d P B4", gameID)},
	}

	for _, m := range moves {
		c1.Send(m[0]);
		c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
		c1.Expect("OK");
		c2.DrainUntil(fmt.Sprintf("Game#%d P", gameID))
		c2.Send(m[1]);
		c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID));
		c2.Expect("OK");
		c1.DrainUntil(fmt.Sprintf("Game#%d P", gameID))
	}

	// White completes the road
	c1.Send(fmt.Sprintf("Game#%d P A4", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))

	c1.DrainUntil(fmt.Sprintf("Game#%d Over R-0", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d Over R-0", gameID))
}

func TestGameResignWhiteLoses(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d P", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d P", gameID))

	// c1 is white — white resigns = black wins = 0-1
	c1.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Over 0-1", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d Over 0-1", gameID))
}

func TestGameResignBlackLoses(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d P", gameID))

	// c2 is black — black resigns = white wins = 1-0
	c2.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Over 1-0", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d Over 1-0", gameID))
}

// ---- Game list ----

func TestGameListShowsActiveGame(t *testing.T) {
	c1, _, gameID := startGame(t)

	observer := client.New(t, telnetAddr(t))
	client.LoginGuest(t, observer)
	observer.Send("GameList")
	observer.DrainUntil(fmt.Sprintf("GameList Add %d ", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGameListRemoveOnGameEnd(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	observer := client.New(t, telnetAddr(t))
	client.LoginGuest(t, observer)
	observer.DrainUntil("OnlinePlayers ")
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK");
	c2.DrainUntil(fmt.Sprintf("Game#%d P", gameID))
	c1.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c1.DrainUntil("Game#")

	observer.DrainUntil(fmt.Sprintf("GameList Remove %d ", gameID))
}

// ---- Show commands ----

func TestGameShowGame(t *testing.T) {
	c1, _, gameID := startGame(t)
	c1.Send(fmt.Sprintf("Game#%d Show", gameID))
	// Should receive some non-error board state response
	msg := c1.Recv()
	if msg == "NOK" {
		t.Errorf("Show command returned NOK, expected board state")
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestGameShowSquare(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P", gameID))

	c1.Send(fmt.Sprintf("Game#%d Show A1", gameID))
	c1.ExpectPrefix(fmt.Sprintf("Game#%d Show Sq [F]", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}
