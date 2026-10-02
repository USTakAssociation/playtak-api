package tak

import (
	"strconv"
	"strings"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

var std5 = Settings{Size: 5, Pieces: 21, Capstones: 1}

// fromTPS builds a game in the position described by tps (rows plus the
// color to move and move number), with the given reserves. Reserves aren't
// part of TPS, so callers supply them.
func fromTPS(t *testing.T, s Settings, tps string) *Game {
	t.Helper()
	fields := strings.Fields(tps)
	if len(fields) != 3 {
		t.Fatalf("bad TPS %q", tps)
	}
	g := New(s)
	rows := strings.Split(fields[0], "/")
	if len(rows) != g.size {
		t.Fatalf("TPS has %d rows, board is %d", len(rows), g.size)
	}
	for i, row := range rows {
		rank := g.size - 1 - i
		file := 0
		for _, cell := range strings.Split(row, ",") {
			if strings.HasPrefix(cell, "x") {
				n := 1
				if len(cell) > 1 {
					n, _ = strconv.Atoi(cell[1:])
				}
				file += n
				continue
			}
			var stack []Stone
			for _, ch := range cell {
				switch ch {
				case '1':
					stack = append(stack, Stone{Color: White})
				case '2':
					stack = append(stack, Stone{Color: Black})
				case 'S':
					stack[len(stack)-1].Kind = Wall
				case 'C':
					stack[len(stack)-1].Kind = Capstone
				}
			}
			g.pos.stacks[g.index(Square{File: file, Rank: rank})] = stack
			file++
		}
		if file != g.size {
			t.Fatalf("TPS row %q has %d squares", row, file)
		}
	}
	turn, _ := strconv.Atoi(fields[1])
	moveNo, _ := strconv.Atoi(fields[2])
	g.pos.ply = (moveNo-1)*2 + turn - 1
	g.pos.lastReset = g.pos.ply
	if got := g.TPS(); got != tps {
		t.Fatalf("TPS round trip: got %q, want %q", got, tps)
	}
	return g
}

// play applies moves in protocol notation, each by the player to move, and
// fails the test on the first rejected move.
func play(t *testing.T, g *Game, moves ...string) {
	t.Helper()
	for _, m := range moves {
		if err := apply(g, g.Turn(), m); err != nil {
			t.Fatalf("%q: %v", m, err)
		}
	}
}

// apply plays one move in protocol notation for player by.
func apply(g *Game, by Color, move string) error {
	f := strings.Fields(move)
	switch f[0] {
	case "P":
		sq, _ := ParseSquare(f[1])
		kind := Flat
		if len(f) > 2 {
			kind = map[string]Kind{"W": Wall, "C": Capstone}[f[2]]
		}
		return g.Place(by, sq, kind)
	case "M":
		from, _ := ParseSquare(f[1])
		to, _ := ParseSquare(f[2])
		var drops []int
		for _, d := range f[3:] {
			n, _ := strconv.Atoi(d)
			drops = append(drops, n)
		}
		return g.Move(by, from, to, drops)
	}
	panic("bad move " + move)
}

func sq(s string) Square {
	q, ok := ParseSquare(s)
	if !ok {
		panic(s)
	}
	return q
}

func expectErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %q, got none", want)
	}
	if err.Error() != want {
		t.Fatalf("error %q, want %q", err, want)
	}
}

var dbs = Settings{Size: 6, Pieces: 30, Capstones: 1, Opening: protocol.OpeningDoubleBlackStack}
