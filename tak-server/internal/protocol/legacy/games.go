package legacy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

var (
	rePlace      = regexp.MustCompile(`^Game#(\d+) P ([A-Z])(\d)( C)?( W)?`)
	reMove       = regexp.MustCompile(`^Game#(\d+) M ([A-Z])(\d) ([A-Z])(\d)(( \d)+)`)
	reResign     = regexp.MustCompile(`^Game#(\d+) Resign`)
	reShow       = regexp.MustCompile(`^Game#(\d+) Show$`)
	reShowSquare = regexp.MustCompile(`^Game#(\d+) Show ([A-Z])(\d)`)
	reGameList   = regexp.MustCompile(`^GameList`)
)

// parseGameCommand parses in-game commands and GameList. It returns nil if
// line isn't one of them or holds a game number too large to represent.
func parseGameCommand(line string) protocol.Command {
	if m := rePlace.FindStringSubmatch(line); m != nil {
		id, err := strconv.Atoi(m[1])
		if err != nil {
			return nil
		}
		// "C W" together places a capstone, as in Java.
		return protocol.PlaceStone{GameID: id, Square: m[2] + m[3], Capstone: m[4] != "", Wall: m[5] != "" && m[4] == ""}
	}
	if m := reMove.FindStringSubmatch(line); m != nil {
		id, err := strconv.Atoi(m[1])
		if err != nil {
			return nil
		}
		var drops []int
		for _, d := range strings.Fields(m[6]) {
			n, _ := strconv.Atoi(d) // single digits
			drops = append(drops, n)
		}
		return protocol.MoveStack{GameID: id, From: m[2] + m[3], To: m[4] + m[5], Drops: drops}
	}
	if m := reResign.FindStringSubmatch(line); m != nil {
		if id, err := strconv.Atoi(m[1]); err == nil {
			return protocol.Resign{GameID: id}
		}
		return nil
	}
	if m := reShow.FindStringSubmatch(line); m != nil {
		if id, err := strconv.Atoi(m[1]); err == nil {
			return protocol.ShowGame{GameID: id}
		}
		return nil
	}
	if m := reShowSquare.FindStringSubmatch(line); m != nil {
		if id, err := strconv.Atoi(m[1]); err == nil {
			return protocol.ShowSquare{GameID: id, Square: m[2] + m[3]}
		}
		return nil
	}
	if reGameList.MatchString(line) {
		return protocol.ListGames{}
	}
	return nil
}

// formatGameStart renders "Game Start". Protocol 0-1 put the size first and
// omit the increment and flags; protocol 4 adds scaled increment and opening.
func formatGameStart(ev protocol.GameStarted, version int) string {
	g := ev.Game
	color := "white"
	if ev.Color == protocol.Black {
		color = "black"
	}
	if version < 2 {
		return fmt.Sprintf("Game Start %d %d %s vs %s %s %d %d %d %d %d %d",
			g.ID, g.Size, g.White, g.Black, color, g.Time, g.Komi, g.Pieces, g.Capstones, g.ExtraTimeTrigger, g.ExtraTimeAmount)
	}
	f := []string{
		"Game Start", strconv.Itoa(g.ID), g.White, "vs", g.Black, color,
		strconv.Itoa(g.Size), strconv.Itoa(g.Time), strconv.Itoa(g.Increment),
	}
	if version >= 4 {
		f = append(f, flag(g.IncrementScales))
	}
	f = append(f, strconv.Itoa(g.Komi), strconv.Itoa(g.Pieces), strconv.Itoa(g.Capstones),
		flag(g.Unrated), flag(g.Tournament), strconv.Itoa(g.ExtraTimeTrigger), strconv.Itoa(g.ExtraTimeAmount), flag(g.Bot))
	if version >= 4 {
		f = append(f, strconv.Itoa(int(g.Opening)))
	}
	return strings.Join(f, " ")
}

// formatGameListEntry renders the fields after "GameList Add/Remove ".
func formatGameListEntry(g protocol.GameInfo, version int) string {
	f := []string{
		strconv.Itoa(g.ID), g.White, g.Black, strconv.Itoa(g.Size), strconv.Itoa(g.Time), strconv.Itoa(g.Increment),
	}
	if version >= 4 {
		f = append(f, flag(g.IncrementScales))
	}
	f = append(f, strconv.Itoa(g.Komi), strconv.Itoa(g.Pieces), strconv.Itoa(g.Capstones),
		flag(g.Unrated), flag(g.Tournament), strconv.Itoa(g.ExtraTimeTrigger), strconv.Itoa(g.ExtraTimeAmount))
	if version >= 4 {
		f = append(f, strconv.Itoa(int(g.Opening)))
	}
	return strings.Join(f, " ")
}

// formatTime renders remaining time: whole seconds as "Time" for protocol 0,
// milliseconds as "Timems" from protocol 1. Negative values show as 0.
func formatTime(ev protocol.GameTime, version int) string {
	w, b := max(ev.White, 0), max(ev.Black, 0)
	if version >= 1 {
		return fmt.Sprintf("Game#%d Timems %d %d", ev.GameID, w, b)
	}
	return fmt.Sprintf("Game#%d Time %d %d", ev.GameID, w/1000, b/1000)
}
