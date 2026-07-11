package suite_test

import (
	"fmt"
	"testing"
	"time"
	"tak-tests/internal/client"
)

func TestDrawOfferSentToOpponent(t *testing.T) {
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

	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID))
	c1.Expect("OK")
	c2.Expect(fmt.Sprintf("Game#%d OfferDraw", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestDrawAcceptedEndsGame(t *testing.T) {
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

	// c1 offers
	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID))
	c1.Expect("OK")
	c2.Expect(fmt.Sprintf("Game#%d OfferDraw", gameID))

	// c2 accepts by also sending OfferDraw
	c2.Send(fmt.Sprintf("Game#%d OfferDraw", gameID))

	// clean up
	c1.DrainUntil(fmt.Sprintf("Game#%d Over 1/2-1/2", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d Over 1/2-1/2", gameID))
}

func TestDrawResultIsHalfHalf(t *testing.T) {
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

	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID)); 
	c1.Expect("OK");
	c2.Expect(fmt.Sprintf("Game#%d OfferDraw", gameID))
	c2.Send(fmt.Sprintf("Game#%d OfferDraw", gameID));

	// clean up
	over := c1.DrainUntil(fmt.Sprintf("Game#%d Over", gameID))
	if over != fmt.Sprintf("Game#%d Over 1/2-1/2", gameID) {
		t.Errorf("draw result should be 1/2-1/2, got %q", over)
	}
}

func TestDrawRemoveDraw(t *testing.T) {
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

	// c1 offers draw
	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID))
	c1.Expect("OK")
	c2.Expect(fmt.Sprintf("Game#%d OfferDraw", gameID))

	// c1 takes it back
	c1.Send(fmt.Sprintf("Game#%d RemoveDraw", gameID))
	c1.Expect("OK")
	c2.Expect(fmt.Sprintf("Game#%d RemoveDraw", gameID))
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestDrawNotAcceptedBySamePlayer(t *testing.T) {
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

	// c1 offers draw
	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID))
	c1.Expect("OK")
	c2.Expect(fmt.Sprintf("Game#%d OfferDraw", gameID))

	// c1 sends OfferDraw again — should be a no-op, not a self-accept
	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID))
	// Game should NOT be over
	c1.Send("PING")
	c1.Expect("OK")
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestDrawGameListRemovedAfterDraw(t *testing.T) {
	c1, c2, gameID := startGame(t)
	makeOpeningMoves(t, c1, c2, gameID)
	obs := client.New(t, telnetAddr(t))
	//obs login guest
	client.LoginGuest(t, obs)
	obs.DrainUntil("OnlinePlayers")
	c1.Send(fmt.Sprintf("Game#%d P A1", gameID));
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK"); 
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Send(fmt.Sprintf("Game#%d P A2", gameID));
	c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.Expect("OK");
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d", gameID))

	c1.Send(fmt.Sprintf("Game#%d OfferDraw", gameID));
	c1.Expect("OK");
	c2.Expect(fmt.Sprintf("Game#%d OfferDraw", gameID))
	c2.Send(fmt.Sprintf("Game#%d OfferDraw", gameID));

	c1.DrainUntil(fmt.Sprintf("Game#%d Over 1/2-1/2", gameID))

	// A third client should see the game removed from the game list
	obs.Expect(fmt.Sprintf("GameList Remove %d %s %s 5 600 30 0 21 1 0 0 0 0", gameID, c1.Name, c2.Name))
	//_ = obs // game is over, but we just check it doesn't crash
	obs.ExpectNoMessage(500 * time.Millisecond)
}
