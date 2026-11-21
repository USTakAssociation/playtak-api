package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Standard flats and capstones per board size, for games stored with -1.
var (
	standardFlats = [...]int{0, 0, 0, 10, 15, 21, 30, 40, 50}
	standardCaps  = [...]int{0, 0, 0, 0, 0, 1, 1, 2, 2}
)

// gamePTN renders a stored game as PTN, matching the NestJS PTNService
// except that pre-2016 players are no longer shown as "Anon".
func gamePTN(g games.Game) string {
	var b strings.Builder
	header := func(key string, val any) { fmt.Fprintf(&b, "[%s \"%v\"]\n", key, val) }
	date := time.UnixMilli(g.Date).UTC()

	header("Site", "PlayTak.com")
	header("Event", "Online Play")
	header("Date", date.Format("2006.01.02"))
	header("Time", date.Format("15:04:05"))
	header("Player1", g.PlayerWhite)
	if g.RatingWhite != 0 {
		header("Rating1", g.RatingWhite)
	}
	header("Player2", g.PlayerBlack)
	if g.RatingBlack != 0 {
		header("Rating2", g.RatingBlack)
	}
	header("Clock", clockTag(g))
	header("Result", g.Result)
	header("Size", g.Size)
	header("Komi", strconv.FormatFloat(float64(g.Komi)/2, 'f', -1, 64))
	flats, caps := g.Pieces, g.Capstones
	if g.Size >= 0 && g.Size < len(standardFlats) {
		if flats == -1 {
			flats = standardFlats[g.Size]
		}
		if caps == -1 {
			caps = standardCaps[g.Size]
		}
	}
	header("Flats", flats)
	header("Caps", caps)
	if g.Opening != "" && g.Opening != protocol.OpeningSwap.PTN() {
		header("Opening", g.Opening)
	}

	b.WriteString("\n")
	ply := 0
	for move := range strings.SplitSeq(g.Notation, ",") {
		if move == "" {
			continue
		}
		if ply%2 == 0 {
			fmt.Fprintf(&b, "\n%d.", ply/2+1)
		}
		b.WriteString(" ")
		// Double black stack: white's first ply places two black flats.
		if ply == 0 && g.Opening == protocol.OpeningDoubleBlackStack.PTN() {
			b.WriteString("2")
		}
		b.WriteString(ptnMove(move))
		ply++
	}
	b.WriteString("\n" + g.Result + "\n")
	return b.String()
}

// ptnMove converts a stored move ("P A4 C", "M A2 A5 2 1") to PTN ("Ca4",
// "3a2+21"). Anything else becomes "".
func ptnMove(move string) string {
	f := strings.Fields(move)
	switch {
	case len(f) >= 2 && f[0] == "P":
		stone := ""
		if len(f) == 3 {
			stone = "S"
			if f[2] == "C" {
				stone = "C"
			}
		}
		return stone + strings.ToLower(f[1])
	case len(f) >= 3 && f[0] == "M" && len(f[1]) >= 2 && len(f[2]) >= 2:
		from, to := f[1], f[2]
		var dir string
		switch {
		case from[0] == to[0] && to[1] > from[1]:
			dir = "+"
		case from[0] == to[0]:
			dir = "-"
		case to[0] > from[0]:
			dir = ">"
		default:
			dir = "<"
		}
		lifted := 0
		for _, n := range f[3:] {
			v, _ := strconv.Atoi(n)
			lifted += v
		}
		return strconv.Itoa(lifted) + strings.ToLower(from) + dir + strings.Join(f[3:], "")
	}
	return ""
}

// clockTag is the PTN Clock value: "10:0 +5" for 10 minutes plus 5 seconds
// a move, "+1n" for an increment that scales with the move number, and
// "@35 +10:0" for 10 minutes added at move 35.
func clockTag(g games.Game) string {
	val := duration(g.TimerTime)
	if g.TimerInc != 0 {
		val += " +" + strconv.Itoa(g.TimerInc)
		if g.IncrementScales != 0 {
			val += "n"
		}
	}
	if g.ExtraTimeTrigger > 0 && g.ExtraTimeAmount > 0 {
		val += " @" + strconv.Itoa(g.ExtraTimeTrigger) + " +" + duration(g.ExtraTimeAmount)
	}
	return val
}

// duration formats seconds as "h:m:s", dropping leading zero fields and not
// padding: 90 -> "1:30", 600 -> "10:0", 3600 -> "1:0:0".
func duration(seconds int) string {
	hrs, mins, secs := seconds/3600, seconds/60%60, seconds%60
	switch {
	case hrs >= 1:
		return fmt.Sprintf("%d:%d:%d", hrs, mins, secs)
	case mins >= 1:
		return fmt.Sprintf("%d:%d", mins, secs)
	}
	return strconv.Itoa(secs)
}
