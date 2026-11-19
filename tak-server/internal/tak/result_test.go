package tak

import "testing"

func TestRoads(t *testing.T) {
	for _, tc := range []struct {
		name string
		tps  string // position before the deciding move
		move string
		want Result
	}{
		{"horizontal", "x5/x5/x5/x5/1,1,1,1,x 1 10", "P E1", WhiteRoad},
		{"vertical", "x5/2,x4/2,x4/2,x4/2,x4 2 10", "P A5", BlackRoad},
		{"bending", "x5/x5/x,1,1,1,1/x,1,x3/1,x4 1 10", "P B1", WhiteRoad},
		{"winding", "x5/1,1,1,x2/x2,1,x2/x2,1,1,x/x5 1 10", "P E2", WhiteRoad},
		{"through capstone", "x5/x5/x5/x5/1,1C,1,1,x 1 10", "P E1", WhiteRoad},
		{"wall blocks", "x5/x5/x5/x5/1,1S,1,1,x 1 10", "P E1", None},
		{"opponent stone blocks", "x5/x5/x5/x5/1,2,1,1,x 1 10", "P E1", None},
		{"diagonal isn't connected", "x5/x5/x5/x,1,1,1,1/1,x4 1 10", "P A3", None},
		{"edge to same edge isn't a road", "x5/x5/x5/1,1,1,1,x/1,x3,1 1 10", "P C3", None},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := fromTPS(t, std5, tc.tps)
			play(t, g, tc.move)
			if g.Result() != tc.want {
				t.Errorf("result %v, want %v (%s)", g.Result(), tc.want, g.TPS())
			}
		})
	}
}

func TestRoadsOnEveryBoardSize(t *testing.T) {
	for size := 3; size <= 8; size++ {
		g := New(Settings{Size: size, Pieces: 50, Capstones: 2})
		// Fill rank 1 except the last square with white flats.
		for f := 0; f < size-1; f++ {
			g.position.stacks[g.index(Square{File: f, Rank: 0})] = []Stone{{Color: White}}
		}
		g.position.ply = 10
		if err := g.Place(White, Square{File: size - 1, Rank: 0}, Flat); err != nil {
			t.Fatal(err)
		}
		if g.Result() != WhiteRoad {
			t.Errorf("size %d: result %v", size, g.Result())
		}
	}
}

func TestRoadMadeByMoving(t *testing.T) {
	g := fromTPS(t, std5, "x5/x5/x5/x2,1,x2/1,1,x,1,1 1 10")
	play(t, g, "M C2 C1 1")
	if g.Result() != WhiteRoad {
		t.Errorf("result %v", g.Result())
	}
}

func TestOpponentCanCompleteYourRoad(t *testing.T) {
	// Black moves a stack off A5, uncovering white's stone and completing
	// white's road but not black's: white wins even though black moved.
	g := fromTPS(t, std5, "12,1,1,1,1/x5/x5/x5/x5 2 10")
	play(t, g, "M A5 A4 1")
	if g.Result() != WhiteRoad {
		t.Errorf("result %v, want white's road", g.Result())
	}
}

func TestDoubleRoadMoverWins(t *testing.T) {
	// Moving black's stone off A5 reveals white's road on rank 5 while
	// completing black's on rank 4: the mover wins.
	g := fromTPS(t, std5, "12,1,1,1,1/x,2,2,2,2/x5/x5/x5 2 10")
	play(t, g, "M A5 A4 1")
	if g.Result() != BlackRoad {
		t.Errorf("result %v, want the mover (black) to win (%s)", g.Result(), g.TPS())
	}

	g = fromTPS(t, std5, "21,2,2,2,2/x,1,1,1,1/x5/x5/x5 1 10")
	play(t, g, "M A5 A4 1")
	if g.Result() != WhiteRoad {
		t.Errorf("result %v, want the mover (white) to win", g.Result())
	}
}

func TestRoadBeatsFlats(t *testing.T) {
	// White's last placement fills the board and completes a road on rank 1.
	g := fromTPS(t, Settings{Size: 3, Pieces: 10}, "2,1,2/1,2,1/1,1,x 1 10")
	play(t, g, "P C1")
	if g.Result() != WhiteRoad {
		t.Errorf("result %v, want a road win", g.Result())
	}
}

func TestFlatWins(t *testing.T) {
	// Before the move:   after white's P C1, flats on top are
	//   1  2  1           white: A3 C3 B1 C1 (4)
	//   2  1S 2C          black: B3 A2 A1    (3)
	//   2  1  .           walls and capstones don't count.
	const whiteAhead = "1,2,1/2,1S,2C/2,1,x 1 10"
	for _, tc := range []struct {
		name string
		komi int
		tps  string
		move string
		want Result
	}{
		{"white more flats", 0, whiteAhead, "P C1", WhiteFlat},
		{"black more flats", 0, "2,1,2/1,2S,1C/1,2,x 2 10", "P C1", BlackFlat},
		{"equal is a draw", 0, whiteAhead, "P C1 W", Draw},
		{"komi evens it", 2, whiteAhead, "P C1", Draw},
		{"komi wins for black", 4, whiteAhead, "P C1", BlackFlat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := fromTPS(t, Settings{Size: 3, Pieces: 10, Capstones: 1, Komi: tc.komi}, tc.tps)
			play(t, g, tc.move)
			if g.Result() != tc.want {
				t.Errorf("result %v, want %v", g.Result(), tc.want)
			}
		})
	}
}

func TestBoardFilledByMove(t *testing.T) {
	// B1 is black under white; moving the white stone up fills B2, the last
	// empty square. No roads: white 5 flats, black 4.
	g := fromTPS(t, Settings{Size: 3, Pieces: 10}, "1,2,1/2,x,2/1,21,1 1 10")
	play(t, g, "M B1 B2 1")
	if g.Result() != WhiteFlat {
		t.Errorf("result %v, want a flat win (%s)", g.Result(), g.TPS())
	}
}

func TestOutOfPiecesOnlyEndsOnPlacement(t *testing.T) {
	// Both players have no reserves left; a move doesn't trigger the count.
	g := fromTPS(t, Settings{Size: 5}, "x5/x5/x5/x5/1,x,2,x2 1 10")
	play(t, g, "M A1 B1 1")
	if g.Result() != None {
		t.Errorf("result %v, want the game to continue", g.Result())
	}
}

func TestResultStrings(t *testing.T) {
	want := map[Result]string{
		None: "---", WhiteRoad: "R-0", BlackRoad: "0-R", WhiteFlat: "F-0", BlackFlat: "0-F",
		Draw: "1/2-1/2", WhiteWin: "1-0", BlackWin: "0-1", Aborted: "0-0",
	}
	for r, s := range want {
		if r.String() != s {
			t.Errorf("%d.String() = %q, want %q", r, r.String(), s)
		}
	}
	if Win(White) != WhiteWin || Win(Black) != BlackWin {
		t.Error("Win")
	}
}
