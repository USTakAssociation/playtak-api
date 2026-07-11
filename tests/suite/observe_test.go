package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

func TestObserveReceivesObserveMessage(t *testing.T) {
	c1, _, gameID := startGame(t)

	obs := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs)

	obs.Send(fmt.Sprintf("Observe %d", gameID))
	msg := obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))
	if !strings.HasPrefix(msg, fmt.Sprintf("Observe %d ", gameID)) {
		t.Errorf("expected Observe confirmation, got %q", msg)
	}
	// cleanup: end the game so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestObserveReceivesMoveHistory(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	// Play a couple of moves
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")
	c1.DrainUntil(fmt.Sprintf("Game#%d P A2", gameID))

	obs := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs)
	obs.Send(fmt.Sprintf("Observe %d", gameID))

	// Should receive Observe header first
	obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))
	// Then the move history (all prior moves replayed)
	obs.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))
	
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestObserverReceivesLiveMoves(t *testing.T) {
	c1, c2, gameID := startGame(t)

	obs := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs)
	obs.Send(fmt.Sprintf("Observe %d", gameID))
	obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))

	// Now make a move — observer should see it
	makeOpeningMoves(t, c1, c2, gameID)
	obs.DrainUntil(fmt.Sprintf("Game#%d P A5", gameID))
	
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestObserverReceivesTimeUpdates(t *testing.T) {
	c1, c2, gameID := startGameWithProtocol(t, 1)

	obs := client.New(t, telnetAddr(t))
	client.SetProtocol(t, obs, 1)
	client.LoginGuestAfterProtocol(t, obs)
	obs.Send(fmt.Sprintf("Observe %d", gameID))
	// obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))

	makeOpeningMoves(t, c1, c2, gameID)
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")
	c1.DrainUntil(fmt.Sprintf("Game#%d P A2", gameID))

	obs.DrainUntil(fmt.Sprintf("Game#%d Timems", gameID))
	
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestObserverReceivesGameOver(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	obs := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs)
	obs.DrainUntil("OnlinePlayers")
	obs.Send(fmt.Sprintf("Observe %d", gameID))
	obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID))
	c1.DrainUntil("OK")
	c2.DrainUntil(fmt.Sprintf("Game#%d P A1", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c2.DrainUntil("OK")
	c1.DrainUntil(fmt.Sprintf("Game#%d P A2", gameID))

	// c2 resigns
	c2.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c2.DrainUntil("Game#")
	c1.DrainUntil("Game#")

	obs.DrainUntil(fmt.Sprintf("Game#%d Over", gameID))
}

func TestObserveInvalidGameID(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers")
	c.Send("Observe 99999999")
	c.Expect("NOK")
}

func TestObserveUnobservePath(t *testing.T) {
	c1, _, gameID := startGame(t)

	obs := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs)

	obs.Send(fmt.Sprintf("Observe %d", gameID))
	obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))
	obs.DrainUntil("Game#")
	obs.Send(fmt.Sprintf("Unobserve %d", gameID))
	obs.Expect("OK")
	// cleanup active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestObserveUnobserveStopsMoveEchoes(t *testing.T) {
	c1, c2, gameID := startGame(t)

	obs := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs)

	obs.Send(fmt.Sprintf("Observe %d", gameID))
	obs.DrainUntil(fmt.Sprintf("Observe %d ", gameID))
	obs.DrainUntil("Game#")
	obs.Send(fmt.Sprintf("Unobserve %d", gameID))
	obs.Expect("OK")

	// Make a move — observer should NOT see it
	makeOpeningMoves(t, c1, c2, gameID)
	obs.ExpectNoMessage(client.NoMessageWindow)
	
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestObserveMultipleObservers(t *testing.T) {
	c1, c2, gameID := startGame(t)

	obs1 := client.New(t, telnetAddr(t))
	obs2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, obs1)
	client.LoginGuest(t, obs2)

	obs1.Send(fmt.Sprintf("Observe %d", gameID))
	obs1.DrainUntil(fmt.Sprintf("Observe %d ", gameID))
	obs2.Send(fmt.Sprintf("Observe %d", gameID))
	obs2.DrainUntil(fmt.Sprintf("Observe %d ", gameID))

	makeOpeningMoves(t, c1, c2, gameID)

	obs1.DrainUntil(fmt.Sprintf("Game#%d P A5", gameID))
	obs2.DrainUntil(fmt.Sprintf("Game#%d P A5", gameID))
	
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}
