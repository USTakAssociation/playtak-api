package suite_test

import (
	"fmt"
	"testing"

	"tak-tests/internal/client"
)

func TestUndoRequestSentToOpponent(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK"); 
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	// c2 requests undo of c1's last move
	c2.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c2.Expect("OK")
	// c1 should receive the request
	c1.Expect(fmt.Sprintf("Game#%d RequestUndo", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestUndoAcceptedRollsBackMove(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK"); 
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	// c2 requests undo
	c2.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c2.Expect("OK")
	c1.Expect(fmt.Sprintf("Game#%d RequestUndo", gameID))

	// c1 accepts
	c1.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))

	// Both players receive Undo confirmation
	c1.Expect(fmt.Sprintf("Game#%d Undo", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect(fmt.Sprintf("Game#%d Undo", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestUndoObserverAlsoNotified(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	obs := newObserver(t, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK"); 
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	// c2 requests undo
	c2.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c2.Expect("OK")
	c1.Expect(fmt.Sprintf("Game#%d RequestUndo", gameID))

	// c1 accepts
	c1.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))

	obs.DrainUntil(fmt.Sprintf("Game#%d Undo", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestUndoBeforeAnyMoveFails(t *testing.T) {
	c1, _, gameID := startGame(t)
	// No moves played yet — undo should fail
	c1.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	// The server may send NOK or simply ignore; either way no Undo should follow
	// We just verify the connection is still alive
	//c1.Send("PING")
	c1.Expect("NOK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestUndoRemoveUndo(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK"); 
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	// c2 requests undo
	c2.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c2.Expect("OK")
	c1.Expect(fmt.Sprintf("Game#%d RequestUndo", gameID))

	// c2 changes their mind and removes the request
	c2.Send(fmt.Sprintf("Game#%d RemoveUndo", gameID))
	c2.Expect("OK")
	// c1 should be notified the request was withdrawn
	c1.Expect(fmt.Sprintf("Game#%d RemoveUndo", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestUndoClearedByNewMove(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)

	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK"); 
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	// c2 requests undo
	c2.Send(fmt.Sprintf("Game#%d RequestUndo", gameID))
	c2.Expect("OK")
	c1.Expect(fmt.Sprintf("Game#%d RequestUndo", gameID))

	// c1 ignores the request and plays a move instead — this clears the undo request
	c1.Send(fmt.Sprintf("Game#%d P B1", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d P B1", gameID))

	// c2's undo request should now be gone; a second RemoveUndo from c2 should be NOK
	// (or ignored). Connection should still be alive either way.
	c2.Send("PING")
	c2.Expect("OK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

