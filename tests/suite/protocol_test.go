package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// ---- Protocol version command ----

func TestProtocolVersion0IsDefault(t *testing.T) {
	// Without sending Protocol, the server defaults to version 0.
	// Version 0 uses "Time" (seconds) and omits scale_increment from GameList.
	c1, c2, gameID := startGame(t)

	makeOpeningMoves(t, c1, c2, gameID)

	// Next move should produce a Time message (seconds), not Timems
	c1.Send(fmt.Sprintf("Game#%d P A2", gameID))
	c1.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c1.Expect("OK")

	// c2 receives Time first, then the move notification — capture Time here
	timeMsg := c2.DrainUntil(fmt.Sprintf("Game#%d Time", gameID))
	c2.DrainUntil(fmt.Sprintf("Game#%d P A2", gameID))

	if strings.Contains(timeMsg, "Timems") {
		t.Errorf("protocol 0 should use 'Time' not 'Timems', got %q", timeMsg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestProtocolVersion1Accepted(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Protocol 1")
	c.Expect("OK")
}

func TestProtocolVersion2Accepted(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Protocol 2")
	c.Expect("OK")
}

func TestProtocolVersion3Accepted(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Protocol 3")
	c.Expect("OK")
}

func TestProtocolVersion4Accepted(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	c.Expect("Welcome!")
	c.Expect("Login or Register")
	c.Send("Protocol 4")
	c.Expect("OK")
}

// ---- Protocol version affects GameList format ----

func TestProtocolGameListFormatV0to3(t *testing.T) {
	// Protocol 0-3: GameList Add <id> <white> <black> <size> <time> <incr>
	//               <komi> <pieces> <capstones> <unrated> <tournament>
	//               <extra_trigger> <extra_amount>
	// No scale_increment field.
	c1, _, gameID := startGame(t) // default protocol 0

	c3 := client.New(t, telnetAddr(t))
	// No Protocol command — defaults to 0
	client.LoginGuest(t, c3)
	c3.Send("GameList")
	msg := c3.DrainUntil(fmt.Sprintf("GameList Add %d ", gameID))

	parts := strings.Fields(msg)
	// GameList Add <id> <white> <black> <size> <time> <incr> <komi> <pieces>
	// <capstones> <unrated> <tournament> <extra_trigger> <extra_amount>
	// = 15 fields total (including "GameList" and "Add")
	if len(parts) != 15 {
		t.Errorf("protocol 0-3 GameList Add should have 15 fields, got %d: %q", len(parts), msg)
	}
	// clean up active games so the test doesn't leave a dangling game
	client.CleanupActiveGames(t, c1, gameID)
}

func TestProtocolGameListFormatV4(t *testing.T) {
	// Protocol 4: adds scale_increment immediately after incr and appends opening
	// at the end of the protocol-specific GameList Add payload.
	c1, _, gameID := startGameWithProtocol(t, 4)

	c3 := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c3, 4)
	client.LoginGuestAfterProtocol(t, c3)
	c3.Send("GameList")
	msg := c3.DrainUntil(fmt.Sprintf("GameList Add %d ", gameID))

	parts := strings.Fields(msg)
	if len(parts) != 17 {
		t.Errorf("protocol 4 GameList Add should have 17 fields, got %d: %q", len(parts), msg)
	}
	if got, want := parts[8], "1"; got != want {
		t.Errorf("protocol 4 GameList Add scale_increment should be %q at index 8, got %q in %q", want, got, msg)
	}
	if got, want := parts[16], "1"; got != want {
		t.Errorf("protocol 4 GameList Add opening should be %q at index 16, got %q in %q", want, got, msg)
	}
	client.CleanupActiveGames(t, c1, gameID)
}

// ---- Protocol version affects Game Start format ----

func TestProtocolGameStartFormatV0and1(t *testing.T) {
	// Protocol 0-1: Game Start <id> <size> <white> vs <black> <color>
	//               <time> <komi> <pieces> <capstones> <extra_trigger> <extra_amount>
	c := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	// Protocol 0 (default)
	client.LoginGuest(t, c)
	client.LoginGuest(t, c2)

	seekID := postSeek(t, c, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gs := c.DrainUntil("Game Start ")
	parts := strings.Fields(gs)

	// "Game Start <id> <size> <white> vs <black> <color> <time> <komi>
	//  <pieces> <capstones> <extra_trigger> <extra_amount>"
	// = 14 fields
	if len(parts) != 14 {
		t.Errorf("protocol 0-1 Game Start should have 14 fields, got %d: %q", len(parts), gs)
	}
	if parts[0] != "Game" || parts[1] != "Start" {
		t.Errorf("expected 'Game Start ...', got %q", gs)
	}
	client.CleanupActiveGames(t, c, parseGameID(t, gs))
}

func TestProtocolGameStartFormatV2and3(t *testing.T) {
	// Protocol 2-3: Game Start <id> <white> vs <black> <color> <size>
	//               <time> <incr> <komi> <pieces> <capstones> <unrated>
	//               <tournament> <extra_trigger> <extra_amount> <is_bot>
	c := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c, 2)
	client.LoginGuestAfterProtocol(t, c)
	client.SetProtocol(t, c2, 2)
	client.LoginGuestAfterProtocol(t, c2)

	seekID := postSeek(t, c, 5, 600, 30)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gs := c.DrainUntil("Game Start ")
	parts := strings.Fields(gs)

	// "Game Start <id> <white> vs <black> <color> <size> <time> <incr>
	//  <komi> <pieces> <capstones> <unrated> <tournament> <extra_trigger>
	//  <extra_amount> <is_bot>"
	// = 18 fields
	if len(parts) != 18 {
		t.Errorf("protocol 2-3 Game Start should have 18 fields, got %d: %q", len(parts), gs)
	}
	client.CleanupActiveGames(t, c, parseGameID(t, gs))
}

func TestProtocolGameStartFormatV4(t *testing.T) {
	// Protocol 4: adds scale_increment after incr and appends the opening code.
	c := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c, 4)
	client.LoginGuestAfterProtocol(t, c)
	client.SetProtocol(t, c2, 4)
	client.LoginGuestAfterProtocol(t, c2)

	seekID, _ := postSeekV4(t, c, 5, 600, 30, 1, 1)
	c2.Send(fmt.Sprintf("Accept %d", seekID))

	gs := c.DrainUntil("Game Start ")
	parts := strings.Fields(gs)

	if len(parts) != 20 {
		t.Errorf("protocol 4 Game Start should have 20 fields, got %d: %q", len(parts), gs)
	}
	if got, want := parts[10], "1"; got != want {
		t.Errorf("protocol 4 Game Start scale_increment should be %q at index 10, got %q in %q", want, got, gs)
	}
	if got, want := parts[19], "1"; got != want {
		t.Errorf("protocol 4 Game Start opening should be %q at index 19, got %q in %q", want, got, gs)
	}
	client.CleanupActiveGames(t, c, parseGameID(t, gs))
}

// ---- Protocol version affects Seek new/remove format ----

func TestProtocolSeekNewFormatV0and1(t *testing.T) {
	// Protocol 0-1: Seek new <id> <name> <size> <time> <incr> <color>
	//               <komi> <pieces> <capstones> <unrated> <tournament>
	//               <extra_trigger> <extra_amount> <opponent>
	// = 15 fields, no is_bot
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)

	pieces, caps := defaultPieces(5)
	c.Send(fmt.Sprintf("Seek 5 600 30 A 0 %d %d 0 0 ", pieces, caps))
	msg := c.DrainUntil("Seek new ")

	parts := strings.Fields(msg)
	if len(parts) != 15 {
		t.Errorf("protocol 0-1 Seek new should have 15 fields, got %d: %q", len(parts), msg)
	}
}

func TestProtocolSeekNewFormatV2and3(t *testing.T) {
	// Protocol 2-3 adds is_bot at the end = 17 fields
	c := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c, 2)
	client.LoginGuestAfterProtocol(t, c)

	pieces, caps := defaultPieces(5)
	c.Send(fmt.Sprintf("Seek 5 600 30 A 0 %d %d 0 0 ", pieces, caps))
	msg := c.DrainUntil("Seek new ")

	parts := strings.Fields(msg)
	if len(parts) != 17 {
		t.Errorf("protocol 2-3 Seek new should have 17 fields, got %d: %q", len(parts), msg)
	}
}

func TestProtocolSeekNewFormatV4(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.SetProtocol(t, c, 4)
	client.LoginGuestAfterProtocol(t, c)

	_, msg := postSeekV4(t, c, 5, 600, 30, 1, 1)
	parts := strings.Fields(msg)
	if len(parts) != 19 {
		t.Errorf("protocol 4 Seek new should have 19 fields, got %d: %q", len(parts), msg)
	}
	if got, want := parts[7], "1"; got != want {
		t.Errorf("protocol 4 Seek new scale_increment should be %q at index 7, got %q in %q", want, got, msg)
	}
	if got, want := parts[18], "1"; got != want {
		t.Errorf("protocol 4 Seek new opening should be %q at index 18, got %q in %q", want, got, msg)
	}
}
