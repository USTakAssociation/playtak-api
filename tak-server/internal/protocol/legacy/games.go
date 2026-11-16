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

	reRequestUndo = regexp.MustCompile(`^Game#(\d+) RequestUndo`)
	reRemoveUndo  = regexp.MustCompile(`^Game#(\d+) RemoveUndo`)
	reOfferDraw   = regexp.MustCompile(`^Game#(\d+) OfferDraw`)
	reRemoveDraw  = regexp.MustCompile(`^Game#(\d+) RemoveDraw`)
	reGiveTime    = regexp.MustCompile(`^Game#(\d+) GiveTime$`)
	reObserve     = regexp.MustCompile(`^Observe (\d+)`)
	reUnobserve   = regexp.MustCompile(`^Unobserve (\d+)`)
)

// gameOnly maps commands that carry nothing but a game number.
var gameOnly = []struct {
	pattern *regexp.Regexp
	make    func(id int) protocol.Command
}{
	{reResign, func(id int) protocol.Command { return protocol.Resign{GameID: id} }},
	{reShow, func(id int) protocol.Command { return protocol.ShowGame{GameID: id} }},
	{reRequestUndo, func(id int) protocol.Command { return protocol.RequestUndo{GameID: id} }},
	{reRemoveUndo, func(id int) protocol.Command { return protocol.RemoveUndo{GameID: id} }},
	{reOfferDraw, func(id int) protocol.Command { return protocol.OfferDraw{GameID: id} }},
	{reRemoveDraw, func(id int) protocol.Command { return protocol.RemoveDraw{GameID: id} }},
	{reGiveTime, func(id int) protocol.Command { return protocol.GiveTime{GameID: id} }},
	{reObserve, func(id int) protocol.Command { return protocol.Observe{GameID: id} }},
	{reUnobserve, func(id int) protocol.Command { return protocol.Unobserve{GameID: id} }},
}

// parseGameCommand parses in-game commands, GameList and observing. It returns nil if
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
	if match := reMove.FindStringSubmatch(line); match != nil {
		id, err := strconv.Atoi(match[1])
		if err != nil {
			return nil
		}
		var drops []int
		for _, d := range strings.Fields(match[6]) {
			n, _ := strconv.Atoi(d) // single digits
			drops = append(drops, n)
		}
		return protocol.MoveStack{GameID: id, From: match[2] + match[3], To: match[4] + match[5], Drops: drops}
	}
	for _, c := range gameOnly {
		if m := c.pattern.FindStringSubmatch(line); m != nil {
			if id, err := strconv.Atoi(m[1]); err == nil {
				return c.make(id)
			}
			return nil
		}
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
func formatGameStart(event protocol.GameStarted, version int) string {
	info := event.Game
	color := "white"
	if event.Color == protocol.Black {
		color = "black"
	}
	if version < 2 {
		return fmt.Sprintf("Game Start %d %d %s vs %s %s %d %d %d %d %d %d",
			info.ID, info.Size, info.White, info.Black, color, info.Time, info.Komi, info.Pieces, info.Capstones, info.ExtraTimeTrigger, info.ExtraTimeAmount)
	}
	fields := []string{
		"Game Start", strconv.Itoa(info.ID), info.White, "vs", info.Black, color,
		strconv.Itoa(info.Size), strconv.Itoa(info.Time), strconv.Itoa(info.Increment),
	}
	if version >= 4 {
		fields = append(fields, flag(info.IncrementScales))
	}
	fields = append(fields, strconv.Itoa(info.Komi), strconv.Itoa(info.Pieces), strconv.Itoa(info.Capstones),
		flag(info.Unrated), flag(info.Tournament), strconv.Itoa(info.ExtraTimeTrigger), strconv.Itoa(info.ExtraTimeAmount), flag(info.Bot))
	if version >= 4 {
		fields = append(fields, strconv.Itoa(int(info.Opening)))
	}
	return strings.Join(fields, " ")
}

// formatGameListEntry renders the fields after "GameList Add/Remove ".
func formatGameListEntry(info protocol.GameInfo, version int) string {
	fields := []string{
		strconv.Itoa(info.ID), info.White, info.Black, strconv.Itoa(info.Size), strconv.Itoa(info.Time), strconv.Itoa(info.Increment),
	}
	if version >= 4 {
		fields = append(fields, flag(info.IncrementScales))
	}
	fields = append(fields, strconv.Itoa(info.Komi), strconv.Itoa(info.Pieces), strconv.Itoa(info.Capstones),
		flag(info.Unrated), flag(info.Tournament), strconv.Itoa(info.ExtraTimeTrigger), strconv.Itoa(info.ExtraTimeAmount))
	if version >= 4 {
		fields = append(fields, strconv.Itoa(int(info.Opening)))
	}
	return strings.Join(fields, " ")
}

// formatTime renders remaining time: whole seconds as "Time" for protocol 0,
// milliseconds as "Timems" from protocol 1. Negative values show as 0.
func formatTime(event protocol.GameTime, version int) string {
	w, b := max(event.White, 0), max(event.Black, 0)
	if version >= 1 {
		return fmt.Sprintf("Game#%d Timems %d %d", event.GameID, w, b)
	}
	return fmt.Sprintf("Game#%d Time %d %d", event.GameID, w/1000, b/1000)
}
