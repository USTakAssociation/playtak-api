package session

import (
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

// startGame logs in two guests, has the first post a white 5x5 seek and the
// second accept it, and drains everything up to Game Start.
func startGame(h *harness) (white, black *client) {
	white, black = h.connect(), h.connect()
	white.send("Login Guest")
	white.drainUntil("OnlinePlayers")
	black.send("Login Guest")
	black.drainUntil("OnlinePlayers")
	white.drainUntil("OnlinePlayers")
	white.send("Seek 5 600 30 W 0 21 1 0 0 ")
	white.drainUntil("Seek new")
	black.drainUntil("Seek new")
	black.send("Accept 1")
	white.drainUntil("Game Start")
	black.drainUntil("Game Start")
	return white, black
}

func TestGameStartMessages(t *testing.T) {
	h := newHarness(t, Config{})
	a, b := h.connect(), h.connect()
	a.send("Login Guest")
	a.drainUntil("OnlinePlayers")
	b.send("Protocol 4")
	b.expect("OK")
	b.send("Login Guest")
	b.drainUntil("OnlinePlayers")
	a.drainUntil("OnlinePlayers")

	a.send("Seek 5 600 30 W 0 21 1 0 0 ")
	a.expect("Seek new 1 Guest1 5 600 30 W 0 21 1 0 0 0 0 ")
	b.expect("Seek new 1 Guest1 5 600 30 0 W 0 21 1 0 0 0 0 0 0 0")
	b.send("Accept 1")

	// The seek comes down, the game list gains the game, then Game Start in
	// each player's protocol. There's no OK for Accept.
	a.expect("Seek remove 1 Guest1 5 600 30 W 0 21 1 0 0 0 0 ")
	a.expect("GameList Add 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	a.expect("Game Start 1 5 Guest1 vs Guest2 white 600 0 21 1 0 0")
	b.expect("Seek remove 1 Guest1 5 600 30 0 W 0 21 1 0 0 0 0 0 0 0")
	b.expect("GameList Add 1 Guest1 Guest2 5 600 30 0 0 21 1 0 0 0 0 0")
	b.expect("Game Start 1 Guest1 vs Guest2 black 5 600 30 0 0 21 1 0 0 0 0 0 0")
	a.expectNothing()
	b.expectNothing()
}

func TestGamePlay(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)

	w.send("Game#1 P A1")
	w.expect("Game#1 Time 630 600")
	w.expect("OK")
	b.expect("Game#1 Time 630 600")
	b.expect("Game#1 P A1")

	b.send("Game#1 P A1")
	b.expect("NOK")
	b.expect("Error:That square is not empty")
	b.send("Game#9 P B1") // not their game
	b.expect("NOK")
	b.send("Game#1 P E5")
	b.drainUntil("OK")
	w.drainUntil("Game#1 P E5")

	w.send("Game#1 Show")
	w.expect("Game#1 Show x4,1/x5/x5/x5/2,x4 1 2")
	w.send("Game#1 Show A1")
	w.expect("Game#1 Show Sq [f]")
	w.send("Game#1 M A1 A2 1")
	w.expect("NOK")
	w.expect("Error:You don't control that stack")

	// No seeking while playing.
	w.send("Seek 5 600 30")
	w.expect("NOK")
	w.send("Rematch 1 5 600 30 A 0 21 1 0 0 0 0 Guest2")
	w.expect("NOK")
	w.send("Accept 1")
	w.expect("NOK")
}

func TestGameResign(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	w.send("Game#1 Resign")
	w.expect("Game#1 Over 0-1")
	w.expect("GameList Remove 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	b.expect("Game#1 Over 0-1")
	b.expect("GameList Remove 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	w.expectNothing()

	// The result reaches the games DB.
	waitForResult(t, h, 1, "0-1")

	// Free to seek again.
	w.send("Seek 5 600 30")
	w.drainUntil("Seek new")
}

func TestRoadWinThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	for i, sq := range []string{"E5", "A1", "B1", "A5", "C1", "B5", "D1", "C5"} {
		mover, other := w, b
		if i%2 == 1 {
			mover, other = b, w
		}
		mover.send("Game#1 P " + sq)
		mover.drainUntil("OK")
		other.drainUntil("Game#1 P " + sq)
	}
	w.send("Game#1 P E1")
	w.drainUntil("Game#1 Time")
	w.expect("Game#1 Over R-0")
	w.expect("GameList Remove 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	w.expect("OK")
	b.drainUntil("Game#1 P E1")
	b.expect("Game#1 Over R-0")
	waitForResult(t, h, 1, "R-0")
	var notation string
	h.gamesDB.QueryRow(`SELECT notation FROM games WHERE id = 1`).Scan(&notation)
	if notation != "P E5,P A1,P B1,P A5,P C1,P B5,P D1,P C5,P E1" {
		t.Errorf("notation %q", notation)
	}
}

func TestRejoinAfterLoggingInAgain(t *testing.T) {
	h := newHarness(t, Config{})
	h.addPlayer("alice", false)
	h.addPlayer("bobby", false)
	a, b := h.connect(), h.connect()
	a.send("Login alice password")
	a.drainUntil("OnlinePlayers")
	b.send("Login bobby password")
	b.drainUntil("OnlinePlayers")
	a.drainUntil("OnlinePlayers")
	a.send("Seek 5 600 30 W 0 21 1 0 0 ")
	b.drainUntil("Seek new")
	b.send("Accept 1")
	a.drainUntil("Game Start")
	b.drainUntil("Game Start")
	a.send("Game#1 P A1")
	a.drainUntil("OK")
	b.drainUntil("Game#1 P A1")

	a2 := h.connect()
	a2.send("Login alice password")
	a.expect("Message You've logged in from another window. Disconnecting")
	a.expectClosed(transport.CloseKicked)
	b.expect("Message alice has disconnected. They have 60 seconds to reconnect")

	// Registered players resume after Welcome; then the seek and game lists.
	a2.expect("Welcome alice!")
	a2.expect("Game Start 1 5 alice vs bobby white 600 0 21 1 0 0")
	a2.expect("Game#1 P A1")
	a2.drainUntil("Game#1 Time")
	a2.expect("Message Your game is resumed")
	a2.expect("GameList Add 1 alice bobby 5 600 30 0 21 1 0 0 0 0")
	a2.expect("Online 2")
	b.drainUntil("Message alice has reconnected.")

	// The game carries on from the new connection.
	b.send("Game#1 P E5")
	b.drainUntil("OK")
	a2.drainUntil("Game#1 P E5")
}

func TestGuestRejoinsBeforeWelcome(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := h.connect(), h.connect()
	w.send("Login Guest abcdefghijklmnopqrst")
	w.drainUntil("OnlinePlayers")
	b.send("Login Guest")
	b.drainUntil("OnlinePlayers")
	w.send("Seek 5 600 30 W 0 21 1 0 0 ")
	b.drainUntil("Seek new")
	b.send("Accept 1")
	w.drainUntil("Game Start")

	w.send("quit")
	w.expectClosed(transport.CloseNormal)
	b.drainUntil("Message Guest1 has disconnected")

	w2 := h.connect()
	w2.send("Login Guest abcdefghijklmnopqrst")
	w2.expect("Game Start 1 5 Guest1 vs Guest2 white 600 0 21 1 0 0")
	w2.drainUntil("Message Your game is resumed")
	w2.expect("Welcome Guest1!")
}

func TestAbandonedGame(t *testing.T) {
	h := newHarnessWithGames(t, Config{}, game.Config{ReconnectTimeout: 200 * time.Millisecond, CommandTimeout: 5 * time.Second})
	w, b := startGame(h)
	b.send("quit")
	b.expectClosed(transport.CloseNormal)
	w.drainUntil("Message Guest2 has disconnected. They have 0 seconds to reconnect")
	w.expect("Online 1")
	w.expect(`OnlinePlayers ["Guest1"]`)
	w.expect("Game#1 Abandoned. Guest2 quit")
	w.expect("GameList Remove 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	waitForResult(t, h, 1, "1-0")
}

func waitForResult(t *testing.T, h *harness, id int, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var result string
		h.gamesDB.QueryRow(`SELECT result FROM games WHERE id = ?`, id).Scan(&result)
		if result == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("game %d result %q, want %q", id, result, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDrawThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	w.send("Game#9 OfferDraw") // not their game
	w.expect("NOK")
	w.send("Game#1 OfferDraw")
	w.expect("OK")
	b.expect("Game#1 OfferDraw")
	w.send("Game#1 RemoveDraw")
	w.expect("OK")
	b.expect("Game#1 RemoveDraw")
	w.send("Game#1 OfferDraw")
	w.expect("OK")
	b.expect("Game#1 OfferDraw")

	// Agreeing ends the game; the OK comes after the result, as in Java.
	b.send("Game#1 OfferDraw")
	b.expect("Game#1 Over 1/2-1/2")
	b.expect("GameList Remove 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	b.expect("OK")
	w.expect("Game#1 Over 1/2-1/2")
	w.expect("GameList Remove 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	w.expectNothing()
	waitForResult(t, h, 1, "1/2-1/2")
}

func TestUndoThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	w.send("Game#1 RequestUndo")
	w.expect("NOK") // nothing to undo, and no OK after it
	w.expectNothing()

	w.send("Game#1 P A1")
	w.drainUntil("OK")
	b.drainUntil("Game#1 P A1")
	w.send("Game#1 RequestUndo")
	w.expect("OK")
	b.expect("Game#1 RequestUndo")
	w.send("Game#1 RemoveUndo")
	w.expect("OK")
	b.expect("Game#1 RemoveUndo")
	b.send("Game#1 RequestUndo")
	b.expect("OK")
	w.expect("Game#1 RequestUndo")

	w.send("Game#1 RequestUndo")
	w.expect("Game#1 Time 600 600")
	w.expect("Game#1 Undo")
	w.expect("OK")
	b.expect("Game#1 Time 600 600")
	b.expect("Game#1 Undo")
	w.send("Game#1 Show")
	w.expect("Game#1 Show x5/x5/x5/x5/x5 1 1")
}

func TestGiveTimeThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	w.send("Game#1 GiveTime")
	w.expect("NOK")
	w.expect("Error:The clock has not started")

	w.send("Game#1 P A1")
	w.drainUntil("OK")
	b.drainUntil("Game#1 P A1")
	w.send("Game#1 GiveTime")
	w.expect("Game#1 GivenTime black 15000")
	w.drainUntil("Game#1 Time 630 615")
	w.expect("OK")
	b.expect("Game#1 GivenTime black 15000")
	b.drainUntil("Game#1 Time 630 6")

	w.send("Game#1 GiveTime")
	w.expect("Message You can give time again in 15 seconds")
	w.expect("NOK")
	b.expectNothing()
}

func TestRejoinResendsDrawOffer(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := h.connect(), h.connect()
	w.send("Login Guest abcdefghijklmnopqrst")
	w.drainUntil("OnlinePlayers")
	b.send("Login Guest")
	b.drainUntil("OnlinePlayers")
	w.send("Seek 5 600 30 W 0 21 1 0 0 ")
	b.drainUntil("Seek new")
	b.send("Accept 1")
	w.drainUntil("Game Start")
	b.drainUntil("Game Start")
	b.send("Game#1 OfferDraw")
	b.expect("OK")
	w.expect("Game#1 OfferDraw")

	w.send("quit")
	w.expectClosed(transport.CloseNormal)
	w2 := h.connect()
	w2.send("Login Guest abcdefghijklmnopqrst")
	w2.drainUntil("Message Your game is resumed")
	w2.expect("Game#1 OfferDraw")
	w2.expect("Welcome Guest1!")
}

// login connects a guest with the given protocol and drains the login.
func loginGuestWith(h *harness, version string) *client {
	c := h.connect()
	if version != "" {
		c.send("Protocol " + version)
		c.expect("OK")
	}
	c.send("Login Guest")
	c.drainUntil("OnlinePlayers")
	return c
}

func TestObserveThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	w.send("Game#1 P A1")
	w.drainUntil("OK")
	b.drainUntil("Game#1 P A1")

	// Each observer gets the game in its own protocol.
	old, modern := loginGuestWith(h, ""), loginGuestWith(h, "4")
	old.drainUntil("OnlinePlayers") // modern's login
	old.send("Observe 1")
	old.expect("Observe 1 Guest1 Guest2 5 600 30 0 21 1 0 0 0 0")
	old.expect("Game#1 P A1")
	old.drainUntil("Game#1 Time 630 6")
	modern.send("Observe 1")
	modern.expect("Observe 1 Guest1 Guest2 5 600 30 0 0 21 1 0 0 0 0 0")
	modern.expect("Game#1 P A1")
	modern.drainUntil("Game#1 Timems 630000 ")

	b.send("Game#1 P E5")
	b.drainUntil("OK")
	old.drainUntil("Game#1 Time ")
	old.expect("Game#1 P E5")
	modern.drainUntil("Game#1 Timems ")
	modern.expect("Game#1 P E5")

	modern.send("Unobserve 1")
	modern.expect("OK")
	w.send("Game#1 Resign")
	w.drainUntil("Game#1 Over 0-1")
	old.expect("Game#1 Over 0-1")
	modern.drainUntil("GameList Remove 1")
	modern.expectNothing()

	old.send("Unobserve 1") // the game has ended
	old.drainUntil("NOK")
	old.send("Observe 1")
	old.expect("NOK")
}

func TestObserveRefusedThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, _ := startGame(h)
	w.send("Observe 1") // own game
	w.expect("NOK")
	w.send("Observe 99")
	w.expect("NOK")
	w.send("Unobserve 99")
	w.expect("NOK")
}

func TestStartingGameStopsObservingThroughProtocol(t *testing.T) {
	h := newHarness(t, Config{})
	w, b := startGame(h)
	p, q := loginGuestWith(h, ""), loginGuestWith(h, "")
	p.send("Observe 1")
	p.drainUntil("Game#1 Time")
	p.send("Seek 5 600 30")
	q.drainUntil("Seek new")
	q.send("Accept 2")
	p.drainUntil("Game Start 2")
	q.drainUntil("Game Start 2")

	p.send("Observe 1")
	p.expect("NOK") // can't observe while playing
	w.send("Game#1 P A1")
	w.drainUntil("OK")
	b.drainUntil("Game#1 P A1")
	p.expectNothing()
}

func TestDisconnectStopsObserving(t *testing.T) {
	h := newHarness(t, Config{})
	startGame(h)
	o := loginGuestWith(h, "")
	o.send("Observe 1")
	o.drainUntil("Game#1 Time")
	s := o.conn.peer.(*Session)
	if got := h.m.games.ObservedGames(s); len(got) != 1 {
		t.Fatalf("observing %v", got)
	}
	o.send("quit")
	o.expectClosed(transport.CloseNormal)
	<-s.Done()
	if got := h.m.games.ObservedGames(s); len(got) != 0 {
		t.Errorf("closed session still observing %v", got)
	}
}

func TestGameStartFailureTellsBothPlayers(t *testing.T) {
	h := newHarness(t, Config{})
	// A games database missing a migration, as when one is skipped.
	if _, err := h.gamesDB.Exec(`ALTER TABLE games DROP COLUMN increment_scales`); err != nil {
		t.Fatal(err)
	}
	w, b := h.connect(), h.connect()
	w.send("Login Guest")
	w.drainUntil("OnlinePlayers")
	b.send("Login Guest")
	b.drainUntil("OnlinePlayers")
	w.drainUntil("OnlinePlayers")
	w.send("Seek 5 600 30 W 0 21 1 0 0 ")
	w.drainUntil("Seek new")
	b.drainUntil("Seek new")

	b.send("Accept 1")
	b.expect("Seek remove 1 Guest1 5 600 30 W 0 21 1 0 0 0 0 ")
	b.expect("Message Could not start the game. Please try again.")
	b.expect("NOK")
	w.expect("Seek remove 1 Guest1 5 600 30 W 0 21 1 0 0 0 0 ")
	w.expect("Message Could not start the game. Please try again.")
	w.expectNothing()
	b.expectNothing()

	// Both are free to seek again.
	w.send("Seek 5 600 30")
	w.drainUntil("Seek new")
}
