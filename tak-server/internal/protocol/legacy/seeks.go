package legacy

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Seek formats, newest first. The first field is the board size; size 0
// cancels the current seek instead.
//
//	V4: size time incr scales color komi pieces caps unrated tournament trigger amount opening opponent
//	V3: size time incr color komi pieces caps unrated tournament trigger amount opponent
//	V2: size time incr color komi pieces caps unrated tournament opponent
//	V1: size time incr [W|B]
var (
	reList   = regexp.MustCompile(`^List`)
	reSeekV4 = regexp.MustCompile(`^Seek (\d) (\d+) (\d+) (0|1) ([WBA]) (\d+) (\d+) (\d+) (0|1) (0|1) (\d+) (\d+) (0|1) ([A-Za-z0-9_]*)`)
	reSeekV3 = regexp.MustCompile(`^Seek (\d) (\d+) (\d+) ([WBA]) (\d+) (\d+) (\d+) (0|1) (0|1) (\d+) (\d+) ([A-Za-z0-9_]*)`)
	reSeekV2 = regexp.MustCompile(`^Seek (\d) (\d+) (\d+) ([WBA]) (\d+) (\d+) (\d+) (0|1) (0|1) ([A-Za-z0-9_]*)`)
	reSeekV1 = regexp.MustCompile(`^Seek (\d) (\d+) (\d+)( [WB])?`)
	reAccept = regexp.MustCompile(`^Accept (\d+)`)
	// Rematch prefixes a seek with the finished game's ID.
	//	V2: id size time incr scales color komi pieces caps unrated tournament trigger amount opening opponent
	//	V1: id size time incr color komi pieces caps unrated tournament trigger amount opponent
	reRematchV2 = regexp.MustCompile(`^Rematch (\d+) (\d+) (\d+) (\d+) (0|1) ([WBA]) (\d+) (\d+) (\d+) (0|1) (0|1) (\d+) (\d+) (0|1) ([A-Za-z0-9_]*)`)
	reRematchV1 = regexp.MustCompile(`^Rematch (\d+) (\d+) (\d+) (\d+) ([WBA]) (\d+) (\d+) (\d+) (0|1) (0|1) (\d+) (\d+) ([A-Za-z0-9_]*)`)
)

// V1 seeks don't carry piece counts, so they get the standard set for the size.
var v1Pieces = map[int]struct{ pieces, capstones int }{
	3: {10, 0}, 4: {15, 0}, 5: {21, 1}, 6: {30, 1}, 7: {40, 2}, 8: {50, 2},
}

// parseSeekCommand parses List, Seek, Accept and Rematch. It returns nil if
// line isn't one of them or holds a number too large to represent.
func parseSeekCommand(line string) protocol.Command {
	if reList.MatchString(line) {
		return protocol.ListSeeks{}
	}
	if match := reSeekV4.FindStringSubmatch(line); match != nil {
		n, ok := ints(match, 1, 2, 3, 6, 7, 8, 11, 12, 13)
		if !ok {
			return nil
		}
		return seekCommand(protocol.GameSettings{
			Size: n[1], Time: n[2], Increment: n[3], IncrementScales: match[4] == "1", Color: color(match[5]),
			Komi: n[6], Pieces: n[7], Capstones: n[8], Unrated: match[9] == "1", Tournament: match[10] == "1",
			ExtraTimeTrigger: n[11], ExtraTimeAmount: n[12], Opening: protocol.Opening(n[13]),
		}, match[14])
	}
	if match := reSeekV3.FindStringSubmatch(line); match != nil {
		n, ok := ints(match, 1, 2, 3, 5, 6, 7, 10, 11)
		if !ok {
			return nil
		}
		return seekCommand(protocol.GameSettings{
			Size: n[1], Time: n[2], Increment: n[3], Color: color(match[4]),
			Komi: n[5], Pieces: n[6], Capstones: n[7], Unrated: match[8] == "1", Tournament: match[9] == "1",
			ExtraTimeTrigger: n[10], ExtraTimeAmount: n[11],
		}, match[12])
	}
	if m := reSeekV2.FindStringSubmatch(line); m != nil {
		n, ok := ints(m, 1, 2, 3, 5, 6, 7)
		if !ok {
			return nil
		}
		return seekCommand(protocol.GameSettings{
			Size: n[1], Time: n[2], Increment: n[3], Color: color(m[4]),
			Komi: n[5], Pieces: n[6], Capstones: n[7], Unrated: m[8] == "1", Tournament: m[9] == "1",
		}, m[10])
	}
	if m := reSeekV1.FindStringSubmatch(line); m != nil {
		n, ok := ints(m, 1, 2, 3)
		if !ok {
			return nil
		}
		std := v1Pieces[n[1]]
		return seekCommand(protocol.GameSettings{
			Size: n[1], Time: n[2], Increment: n[3], Color: color(strings.TrimSpace(m[4])),
			Pieces: std.pieces, Capstones: std.capstones,
		}, "")
	}
	if m := reAccept.FindStringSubmatch(line); m != nil {
		n, ok := ints(m, 1)
		if !ok {
			return nil
		}
		return protocol.AcceptSeek{ID: n[1]}
	}
	if m := reRematchV2.FindStringSubmatch(line); m != nil {
		n, ok := ints(m, 1, 2, 3, 4, 7, 8, 9, 12, 13, 14)
		if !ok {
			return nil
		}
		return protocol.Rematch{GameID: n[1], Opponent: m[15], Settings: protocol.GameSettings{
			Size: n[2], Time: n[3], Increment: n[4], IncrementScales: m[5] == "1", Color: color(m[6]),
			Komi: n[7], Pieces: n[8], Capstones: n[9], Unrated: m[10] == "1", Tournament: m[11] == "1",
			ExtraTimeTrigger: n[12], ExtraTimeAmount: n[13], Opening: protocol.Opening(n[14]),
		}}
	}
	if m := reRematchV1.FindStringSubmatch(line); m != nil {
		n, ok := ints(m, 1, 2, 3, 4, 6, 7, 8, 11, 12)
		if !ok {
			return nil
		}
		return protocol.Rematch{GameID: n[1], Opponent: m[13], Settings: protocol.GameSettings{
			Size: n[2], Time: n[3], Increment: n[4], Color: color(m[5]),
			Komi: n[6], Pieces: n[7], Capstones: n[8], Unrated: m[9] == "1", Tournament: m[10] == "1",
			ExtraTimeTrigger: n[11], ExtraTimeAmount: n[12],
		}}
	}
	return nil
}

func seekCommand(s protocol.GameSettings, opponent string) protocol.Command {
	if s.Size == 0 {
		return protocol.CancelSeek{}
	}
	return protocol.PostSeek{Settings: s, Opponent: opponent}
}

// ints parses the numeric submatches at the given indexes, keyed by index.
func ints(m []string, idx ...int) (map[int]int, bool) {
	out := make(map[int]int, len(idx))
	for _, i := range idx {
		v, err := strconv.Atoi(m[i])
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

func color(s string) protocol.Color {
	switch s {
	case "W":
		return protocol.White
	case "B":
		return protocol.Black
	}
	return protocol.AnyColor
}

func colorLetter(c protocol.Color) string {
	switch c {
	case protocol.White:
		return "W"
	case protocol.Black:
		return "B"
	}
	return "A"
}

// formatSeek renders the fields after "Seek new " / "Seek remove ".
func formatSeek(openSeek protocol.Seek, version int) string {
	fields := []string{
		strconv.Itoa(openSeek.ID), openSeek.Creator, strconv.Itoa(openSeek.Size), strconv.Itoa(openSeek.Time), strconv.Itoa(openSeek.Increment),
	}
	if version >= 4 {
		fields = append(fields, flag(openSeek.IncrementScales))
	}
	fields = append(fields, colorLetter(openSeek.Color), strconv.Itoa(openSeek.Komi), strconv.Itoa(openSeek.Pieces), strconv.Itoa(openSeek.Capstones),
		flag(openSeek.Unrated), flag(openSeek.Tournament), strconv.Itoa(openSeek.ExtraTimeTrigger), strconv.Itoa(openSeek.ExtraTimeAmount))
	if version <= 1 {
		// Protocol 0-1 ends with the raw opponent, leaving a trailing space
		// when there is none.
		return strings.Join(append(fields, openSeek.Opponent), " ")
	}
	opponent := openSeek.Opponent
	if opponent == "" {
		opponent = "0"
	}
	fields = append(fields, opponent, flag(openSeek.Bot))
	if version >= 4 {
		fields = append(fields, strconv.Itoa(int(openSeek.Opening)))
	}
	return strings.Join(fields, " ")
}

func flag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
