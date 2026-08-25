package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// ---- Posting seeks ----

func TestSeekV1Post(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	// V1: size time incr [color]
	c.Send("Seek 5 600 30")
	msg := c.DrainUntil("Seek new ")
	if !strings.Contains(msg, " 5 ") {
		t.Errorf("seek message missing board size 5: %q", msg)
	}
}

func TestSeekV1WithColor(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.Send("Seek 5 600 30 W")
	msg := c.DrainUntil("Seek new ")
	if !strings.Contains(msg, " W ") {
		t.Errorf("seek message should contain color W: %q", msg)
	}
}

func TestSeekV2Post(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	// V2: size time incr color komi pieces capstones unrated tournament opponent
	c.Send("Seek 6 900 15 A 4 30 1 0 0 ")
	c.DrainUntil("Seek new ")
}

func TestSeekV2AllBoardSizes(t *testing.T) {
	sizes := []int{3, 4, 5, 6, 7, 8}
	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("size%d", size), func(t *testing.T) {
			c := client.New(t, telnetAddr(t))
			client.LoginGuest(t, c)
			pieces, caps := defaultPieces(size)
			c.Send(fmt.Sprintf("Seek %d 600 30 A 0 %d %d 0 0 ", size, pieces, caps))
			c.DrainUntil("Seek new ")
		})
	}
}

func TestSeekV3WithExtraTime(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	// V3: size time incr color komi pieces capstones unrated tournament extra_trigger extra_amount opponent
	c.Send("Seek 5 600 30 A 0 21 1 0 0 20 60 ")
	msg := c.DrainUntil("Seek new ")
	// extra_trigger=20, extra_amount=60 should appear in the message
	if !strings.Contains(msg, " 20 ") {
		t.Errorf("seek message should contain extra_trigger 20: %q", msg)
	}
}

func TestSeekV4WithScaleIncrement(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c, 4)
	client.LoginGuestAfterProtocol(t, c)
	// V4: size time incr scale_increment color komi pieces capstones unrated tournament extra_trigger extra_amount opening opponent
	c.Send("Seek 5 600 30 1 A 0 21 1 0 0 0 0 0 ")
	msg := c.DrainUntil("Seek new ")
	parts := strings.Fields(msg)
	if len(parts) != 19 {
		t.Fatalf("protocol 4 Seek new should have 19 fields, got %d: %q", len(parts), msg)
	}
	if got, want := parts[7], "1"; got != want {
		t.Errorf("scale_increment at index 7 should be %q, got %q in %q", want, got, msg)
	}
}

func TestSeekUnrated(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.Send("Seek 5 600 30 A 0 21 1 1 0 ")
	msg := c.DrainUntil("Seek new ")
	parts := strings.Fields(msg)
	if len(parts) != 15 {
		t.Fatalf("protocol 0-1 Seek new should have 15 fields, got %d: %q", len(parts), msg)
	}
	if got, want := parts[11], "1"; got != want {
		t.Errorf("unrated flag at index 11 should be %q, got %q in %q", want, got, msg)
	}
}

func TestSeekTournament(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.Send("Seek 5 600 30 A 0 21 1 0 1 ")
	c.DrainUntil("Seek new ")
}

// ---- Removing seeks ----

func TestSeekRemoveWithZeroSize(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)

	c.Send("Seek 5 600 30")
	c.DrainUntil("Seek new ")

	// Sending size=0 removes the current seek
	c.Send("Seek 0 0 0")
	c.DrainUntil("Seek remove ")
}

func TestSeekReplacedByNewSeek(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)

	// Post first seek
	c.Send("Seek 5 600 30")
	c.DrainUntil("Seek new ")

	// Posting a new seek should remove the old one first
	c.Send("Seek 6 900 0")
	c.DrainUntil("Seek remove ")
	c.DrainUntil("Seek new ")
}

// ---- Listing seeks ----

func TestListSeeks(t *testing.T) {
	poster := client.New(t, telnetAddr(t))
	lister := client.New(t, telnetAddr(t))
	client.LoginGuest(t, poster)
	client.LoginGuest(t, lister)

	poster.Send("Seek 5 600 30")
	poster.DrainUntil("Seek new ")

	lister.Send("List")
	lister.DrainUntil("Seek new ")
}

func TestListSeeksEmpty(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers ")
	// With no seeks posted, List should return nothing (or return quickly)
	// We just check the server doesn't crash or send NOK
	c.Send("List")
	// Server may send nothing or an empty list — just ensure no error
	c.ExpectNoMessage(client.NoMessageWindow)
}

// ---- Accepting seeks ----

func TestSeekAccept(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gs1 := c1.DrainUntil("Game Start ")
	gameID := parseGameID(t, gs1)
	c2.DrainUntil("Game Start ")

	// clean up
	client.CleanupActiveGames(t, c1, gameID)
}

func TestSeekAcceptRemovesAllSeeks(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	seekID := postSeek(t, c1, 5, 600, 30)

	c2.Send(fmt.Sprintf("Accept %d", seekID))
	c1.DrainUntil("Seek remove ")
	gs1 := c1.DrainUntil("Game Start ")
	gameID := parseGameID(t, gs1)

	// clean up
	client.CleanupActiveGames(t, c1, gameID)
}

func TestSeekAcceptInvalidID(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers")
	c.Send("Accept 99999999")
	c.Expect("NOK")
}

func TestSeekAcceptOwnSeek(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)

	seekID := postSeek(t, c, 5, 600, 30)
	c.Send(fmt.Sprintf("Accept %d", seekID))
	c.Expect("NOK")
}

// ---- Private seeks ----

func TestSeekPrivateSeekOnlyOpponentCanAccept(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	c3 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)
	client.LoginGuest(t, c3)
	c3.DrainUntil("OnlinePlayers ")
	pieces, caps := defaultPieces(5)
	// Seek addressed to u2 only
	c1.Send(fmt.Sprintf("Seek 5 600 30 A 0 %d %d 0 0 %s", pieces, caps, u2))
	seekMsg := c1.DrainUntil("Seek new ")
	seekID := parseSeekID(t, seekMsg)
	c3.DrainUntil(fmt.Sprintf("Seek new %d", seekID)) // c3 sees the seek broadcast, but can't accept
	// c3 (a random guest) should be rejected
	c3.Send(fmt.Sprintf("Accept %d", seekID))
	c3.Expect("NOK")

	// c2 (the named opponent) should succeed
	c2.Send(fmt.Sprintf("Accept %d", seekID))
	gameId := c2.DrainUntil("Game Start ")
	// clean up
	client.CleanupActiveGames(t, c1, parseGameID(t, gameId))
}

// ---- Rematch ----

func TestSeekRematchCreatesSeek(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	gameID := parseGameID(t, gs1)

	// Play through and resign to end game
	makeOpeningMoves(t, c1, c2, gameID)
	c1.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c1.DrainUntil("Game#")
	c2.DrainUntil("Game#")
	c1.DrainUntil("GameList")
	// c1 sends a rematch — should create a seek addressed to c2
	// V1 rematch: Rematch <gameid> <size> <time> <incr> <color> <komi> <pieces> <capstones> <unrated> <tournament> <extra_trigger> <extra_amount> <opponent>
	c1.Send(fmt.Sprintf("Rematch %d 5 600 30 A 0 21 1 0 0 0 0 %s", gameID, u2))
	c1.DrainUntil("Seek new ")
	c1.ExpectPrefix("Rematch seek created with ID:")
}

func TestSeekRematchBothPlayersAccepts(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	seekID := postSeek(t, c1, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gs1 := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	gameID := parseGameID(t, gs1)

	makeOpeningMoves(t, c1, c2, gameID)
	c1.Send(fmt.Sprintf("Game#%d Resign", gameID))
	c1.DrainUntil("Game#")
	c2.DrainUntil("Game#")
	c1.DrainUntil("GameList")
	// Both players send rematch — second one should receive "Accept Rematch <id>"
	c1.Send(fmt.Sprintf("Rematch %d 5 600 30 A 0 21 1 0 0 0 0 %s", gameID, u2))
	c1.DrainUntil("Seek new ")
	c1.ExpectPrefix("Rematch seek created with ID:")
	// c2 needs to check the most recent Seek new and if it is the same as our name and the game size matches the last game then send accept rematch
	c2.DrainUntil("Seek new ")
	c2.Send(fmt.Sprintf("Rematch %d 5 600 30 A 0 21 1 0 0 0 0 %s", gameID, u1))
	newSeekId := parseRematchSeekID(t, c2.ExpectPrefix("Accept Rematch "))
	// accept the new seek
	c2.Send(fmt.Sprintf("Accept %d", newSeekId))
	c1.DrainUntil("Seek remove ")
	newGameId := c1.DrainUntil("Game Start ")
	c2.DrainUntil("Game Start ")
	// clean up game
	client.CleanupActiveGames(t, c1, parseGameID(t, newGameId))
}

