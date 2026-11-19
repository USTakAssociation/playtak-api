package tak

import (
	"math"
	"slices"
	"testing"
)

func TestNewBoardSizeFallback(t *testing.T) {
	for _, size := range []int{0, 2, 9, -1} {
		if g := New(Settings{Size: size}); g.Size() != 5 {
			t.Errorf("size %d became %d, want 5", size, g.Size())
		}
	}
	if g := New(Settings{Size: 8}); g.Size() != 8 {
		t.Errorf("size 8 became %d", g.Size())
	}
}

func TestOpeningPlacesOpponentStones(t *testing.T) {
	g := New(std5)
	play(t, g, "P A1")
	if got := g.SquareString(sq("A1")); got != "[f]" {
		t.Errorf("white's first stone should be black: %s", got)
	}
	play(t, g, "P E5")
	if got := g.SquareString(sq("E5")); got != "[F]" {
		t.Errorf("black's first stone should be white: %s", got)
	}
	// Each stone comes out of the mover's reserve.
	for _, c := range []Color{White, Black} {
		if stones, caps := g.Reserves(c); stones != 20 || caps != 1 {
			t.Errorf("%v reserves = %d, %d", c, stones, caps)
		}
	}
	play(t, g, "P B1")
	if got := g.SquareString(sq("B1")); got != "[F]" {
		t.Errorf("after the opening stones are your own: %s", got)
	}
}

func TestOpeningRules(t *testing.T) {
	g := New(std5)
	expectErr(t, g.Place(White, sq("A1"), Wall), "The first move must place a flat")
	expectErr(t, g.Place(White, sq("A1"), Capstone), "The first move must place a flat")
	expectErr(t, g.Move(White, sq("A1"), sq("A2"), []int{1}), "The first move must be a placement")
	play(t, g, "P A1")
	expectErr(t, g.Place(Black, sq("B1"), Wall), "The first move must place a flat")
	expectErr(t, g.Move(Black, sq("A1"), sq("A2"), []int{1}), "The first move must be a placement")
}

func TestPlaceErrors(t *testing.T) {
	g := New(std5)
	expectErr(t, g.Place(Black, sq("A1"), Flat), "Not your turn")
	expectErr(t, g.Place(White, Square{File: 5, Rank: 0}, Flat), "That square is off the board")
	expectErr(t, g.Place(White, Square{File: 0, Rank: -1}, Flat), "That square is off the board")
	play(t, g, "P A1", "P E5")
	expectErr(t, g.Place(White, sq("A1"), Flat), "That square is not empty")
	play(t, g, "P C3 C", "P C4")
	expectErr(t, g.Place(White, sq("D4"), Capstone), "You have no capstones left")
	if g.Ply() != 4 {
		t.Errorf("rejected moves must not change the game: ply %d", g.Ply())
	}
}

func TestRunningOutOfStonesAndPieces(t *testing.T) {
	g := New(Settings{Size: 3, Pieces: 1, Capstones: 1})
	play(t, g, "P A1", "P C3")
	expectErr(t, g.Place(White, sq("B2"), Flat), "You have no stones left")
	expectErr(t, g.Place(White, sq("B2"), Wall), "You have no stones left")
	// White's capstone is their last piece, so the game is decided on flats:
	// one each (A1 black, C3 white); the capstone doesn't count.
	play(t, g, "P B2 C")
	if g.Result() != Draw {
		t.Errorf("result %v, want a draw", g.Result())
	}
}

func TestReservesAndMoveList(t *testing.T) {
	g := New(std5)
	play(t, g, "P A1", "P E5", "P B2 W", "P D4 C")
	if s, c := g.Reserves(White); s != 19 || c != 1 {
		t.Errorf("white reserves %d, %d", s, c)
	}
	if s, c := g.Reserves(Black); s != 20 || c != 0 {
		t.Errorf("black reserves %d, %d", s, c)
	}
	want := []string{"P A1", "P E5", "P B2 W", "P D4 C"}
	if got := g.Moves(); !slices.Equal(got, want) {
		t.Errorf("moves %q", got)
	}
}

func TestDoubleBlackStack(t *testing.T) {
	g := New(dbs)
	play(t, g, "P A1")
	if got := g.SquareString(sq("A1")); got != "[f, f]" {
		t.Errorf("A1 = %s, want two black flats", got)
	}
	if s, _ := g.Reserves(White); s != 29 {
		t.Errorf("white stones %d, want 29", s)
	}
	if s, _ := g.Reserves(Black); s != 29 {
		t.Errorf("black stones %d after white's move, want 29", s)
	}
	play(t, g, "P F6")
	if got := g.SquareString(sq("F6")); got != "[F]" {
		t.Errorf("black's first move is a normal swap: %s", got)
	}
	if s, _ := g.Reserves(Black); s != 28 {
		t.Errorf("black stones %d, want 28 after the opening", s)
	}
	if got := g.Moves(); !slices.Equal(got, []string{"P A1", "P F6"}) {
		t.Errorf("moves %q: the extra stone isn't written in notation", got)
	}
	if got := g.TPS(); got != "x5,1/x6/x6/x6/x6/22,x5 1 2" {
		t.Errorf("TPS %q", got)
	}
}

func TestHugeDropsAreRejectedWithoutChange(t *testing.T) {
	// Drops that overflow int when summed must not slip past the carry
	// checks, and nothing may change before the move is rejected.
	g := New(std5)
	g.Place(White, sq("A1"), Flat)
	g.Place(Black, sq("B1"), Flat)
	before := g.TPS()
	for _, drops := range [][]int{
		{math.MaxInt, math.MaxInt, 3},
		{math.MaxInt, 1, 1},
		{math.MinInt, math.MinInt, 1},
	} {
		if err := g.Move(White, sq("B1"), sq("B4"), drops); err == nil {
			t.Errorf("%v: accepted", drops)
		}
	}
	if g.TPS() != before || g.Ply() != 2 || len(g.Moves()) != 2 {
		t.Errorf("rejected moves changed the game: %s, ply %d", g.TPS(), g.Ply())
	}
}

func TestMoveErrors(t *testing.T) {
	// White to move. B2 is a white-topped 3-stack, D2 and D3 black, B3 a
	// white wall, A1 a lone white flat.
	g := fromTPS(t, std5, "x5/x5/x,1S,x,2,x/x,211,x,2,x/1,x4 1 5")
	for _, tc := range []struct {
		from, to string
		drops    []int
		want     string
	}{
		{"B2", "C3", []int{1}, "Moves must be in a straight line"},
		{"A1", "A0", []int{1}, "That square is off the board"},
		{"C2", "C1", []int{1}, "There is no stack on that square"},
		{"D2", "D1", []int{1}, "You don't control that stack"},
		{"B2", "B1", []int{6}, "You can carry at most 5 pieces"},
		{"B2", "B4", []int{3, 3}, "You can carry at most 5 pieces"},
		{"B2", "B1", []int{4}, "That stack only has 3 pieces"},
		{"A1", "A2", []int{2}, "That stack only has 1 piece"},
		{"B2", "E2", []int{1, 1}, "The number of drops must match the distance moved"},
		{"B2", "B2", []int{1}, "The number of drops must match the distance moved"},
		{"B2", "B2", nil, "A move must leave its square"},
		{"B2", "D2", []int{1, 0}, "Each square moved over must get at least one piece"},
		{"B2", "B4", []int{1, 1}, "Can't move onto a wall"},
		{"B2", "B3", []int{1}, "Only a capstone moving alone can flatten a wall"},
	} {
		expectErr(t, g.Move(White, sq(tc.from), sq(tc.to), tc.drops), tc.want)
	}
	expectErr(t, g.Move(Black, sq("D2"), sq("D1"), []int{1}), "Not your turn")
	if g.Ply() != 8 {
		t.Errorf("rejected moves must not change the game: ply %d", g.Ply())
	}
}

func TestCantMoveOntoCapstone(t *testing.T) {
	g := fromTPS(t, std5, "x5/x5/x5/x5/1,2C,x3 1 5")
	expectErr(t, g.Move(White, sq("A1"), sq("B1"), []int{1}), "Can't move onto a capstone")
}

func TestWallRules(t *testing.T) {
	// A1: three white stones with a white capstone on top; B1 a white wall.
	g := fromTPS(t, std5, "x5/x5/x5/x5/111C,1S,x3 1 5")
	expectErr(t, g.Move(White, sq("A1"), sq("C1"), []int{1, 1}), "Can't move onto a wall")
	// The capstone must arrive alone to flatten.
	expectErr(t, g.Move(White, sq("A1"), sq("B1"), []int{2}), "Only a capstone moving alone can flatten a wall")
}

func TestSpread(t *testing.T) {
	// A1 bottom to top: black, white, black, white.
	g := fromTPS(t, std5, "x5/x5/x5/x5/2121,x4 1 5")
	play(t, g, "M A1 D1 1 1 1")
	for square, want := range map[string]string{
		"A1": "[f]", "B1": "[F]", "C1": "[f]", "D1": "[F]",
	} {
		if got := g.SquareString(sq(square)); got != want {
			t.Errorf("%s = %s, want %s", square, got, want)
		}
	}
	if got := g.Moves(); !slices.Equal(got, []string{"M A1 D1 1 1 1"}) {
		t.Errorf("moves %q", got)
	}
}

func TestSpreadDropsBottomOfCarryFirst(t *testing.T) {
	// A1 bottom to top: black, black, white, white. Carrying three and
	// dropping 2 then 1 leaves the lowest carried stones on B1.
	g := fromTPS(t, std5, "x5/x5/x5/x5/2211,x4 1 5")
	play(t, g, "M A1 C1 2 1")
	for square, want := range map[string]string{
		"A1": "[f]", "B1": "[f, F]", "C1": "[F]",
	} {
		if got := g.SquareString(sq(square)); got != want {
			t.Errorf("%s = %s, want %s", square, got, want)
		}
	}
}

func TestSpreadInEveryDirection(t *testing.T) {
	for _, tc := range []struct{ to, mid string }{
		{"E3", "D3"}, {"A3", "B3"}, {"C5", "C4"}, {"C1", "C2"},
	} {
		g := fromTPS(t, std5, "x5/x5/x2,11,x2/x5/x5 1 5")
		play(t, g, "M C3 "+tc.to+" 1 1")
		if g.SquareString(sq(tc.mid)) != "[F]" || g.SquareString(sq(tc.to)) != "[F]" || g.SquareString(sq("C3")) != "[]" {
			t.Errorf("C3 to %s: %s", tc.to, g.TPS())
		}
	}
}

func TestFlattenWall(t *testing.T) {
	// A1: black under a white capstone. B1: a black wall.
	g := fromTPS(t, std5, "x5/x5/x5/x5/21C,2S,x3 1 10")
	g.position.lastReset = 0
	play(t, g, "M A1 B1 1")
	if got := g.SquareString(sq("B1")); got != "[f, C]" {
		t.Errorf("B1 = %s, want the wall flattened under the capstone", got)
	}
	if g.position.lastReset != g.position.ply {
		t.Errorf("flattening should reset the stale counter: lastReset %d, ply %d", g.position.lastReset, g.position.ply)
	}
}

func TestFlattenAtEndOfLongerSpread(t *testing.T) {
	// White carries a flat and the capstone; the capstone alone reaches the wall.
	g := fromTPS(t, std5, "x5/x5/x5/x5/1C,x,2S,x2 1 10")
	expectErr(t, g.Move(White, sq("A1"), sq("C1"), []int{1, 1}), "That stack only has 1 piece")
	g = fromTPS(t, std5, "x5/x5/x5/x5/11C,x,2S,x2 1 10")
	play(t, g, "M A1 C1 1 1")
	if got := g.SquareString(sq("C1")); got != "[f, C]" {
		t.Errorf("C1 = %s", got)
	}
}

// rule is the 50-ply rule written out, so a wrong staleLimit fails the tests
// instead of the tests following it.
const rule = 50

// staleMoves shuffles two lone capstones on A1/A2 and E5/E4 without placing.
var staleMoves = []string{"M A1 A2 1", "M E5 E4 1", "M A2 A1 1", "M E4 E5 1"}

func TestStaleGameDraw(t *testing.T) {
	g := fromTPS(t, std5, "x4,2C/x5/x5/x5/1C,x4 1 10")
	for i := 0; i < rule-1; i++ {
		play(t, g, staleMoves[i%4])
	}
	if g.Result() != None {
		t.Fatalf("drawn after %d plies", rule-1)
	}
	play(t, g, staleMoves[(rule-1)%4])
	if g.Result() != Draw {
		t.Errorf("result %v after %d plies without a placement, want draw", g.Result(), rule)
	}
}

func TestPlacementRestartsStaleCount(t *testing.T) {
	g := fromTPS(t, std5, "x4,2C/x5/x5/x5/1C,x4 1 10")
	for i := 0; i < rule-1; i++ {
		play(t, g, staleMoves[i%4])
	}
	// White's capstone is on A2, black's on E5, and it's black's turn.
	play(t, g, "P C3")
	more := []string{"M A2 A1 1", "M E5 E4 1", "M A1 A2 1", "M E4 E5 1"}
	for i := 0; i < rule-1; i++ {
		play(t, g, more[i%4])
	}
	if g.Result() != None {
		t.Fatalf("result %v: the placement should have restarted the count", g.Result())
	}
	play(t, g, more[(rule-1)%4])
	if g.Result() != Draw {
		t.Errorf("result %v, want a draw %d plies after the placement", g.Result(), rule)
	}
}

func TestUndo(t *testing.T) {
	g := New(std5)
	if g.Undo() {
		t.Error("nothing to undo at the start")
	}
	play(t, g, "P A1", "P E5", "P C3 C", "P D4")
	before := g.TPS()
	play(t, g, "M C3 C4 1")
	if !g.Undo() {
		t.Fatal("undo failed")
	}
	if g.TPS() != before || g.Ply() != 4 || len(g.Moves()) != 4 {
		t.Errorf("after undo: %s ply %d moves %q", g.TPS(), g.Ply(), g.Moves())
	}
	g.Undo() // P D4
	g.Undo() // P C3 C
	if _, caps := g.Reserves(White); caps != 1 {
		t.Errorf("undoing a capstone placement should return it, caps %d", caps)
	}
	if g.Turn() != White {
		t.Errorf("turn %v after undo", g.Turn())
	}
}

func TestUndoRestoresStaleCount(t *testing.T) {
	g := fromTPS(t, std5, "x4,2C/x5/x5/x5/1C,x4 1 10")
	play(t, g, "M A1 A2 1", "P C3")
	g.Undo()
	if g.position.lastReset != g.position.ply-1 {
		t.Errorf("lastReset %d after undoing the placement at ply %d", g.position.lastReset, g.position.ply)
	}
}

func TestDecidedGameCantContinue(t *testing.T) {
	g := fromTPS(t, std5, "x5/x5/x5/x5/1,1,1,1,x 1 10")
	play(t, g, "P E1")
	if g.Result() != WhiteRoad {
		t.Fatalf("result %v", g.Result())
	}
	if g.Undo() {
		t.Error("a decided game can't be undone")
	}
	expectErr(t, g.Place(Black, sq("A5"), Flat), "The game is over")
	expectErr(t, g.Move(Black, sq("A1"), sq("A2"), []int{1}), "The game is over")
}

func TestHistoryIsIndependent(t *testing.T) {
	// Flattening edits a stack in place; the saved position must not change.
	g := fromTPS(t, std5, "x5/x5/x5/x5/2C,1S,x3 2 10")
	play(t, g, "M A1 B1 1")
	g.Undo()
	if got := g.SquareString(sq("B1")); got != "[W]" {
		t.Errorf("B1 after undo = %s, want the wall back", got)
	}
}

func TestStackAndSquareString(t *testing.T) {
	g := fromTPS(t, std5, "x5/x5/x5/x5/12S,2C,x3 1 5")
	if got := g.SquareString(sq("A1")); got != "[F, w]" {
		t.Errorf("A1 = %s", got)
	}
	if got := g.SquareString(sq("B1")); got != "[c]" {
		t.Errorf("B1 = %s", got)
	}
	if got := g.SquareString(sq("C1")); got != "[]" {
		t.Errorf("C1 = %s", got)
	}
	if got := g.SquareString(Square{File: 7, Rank: 7}); got != "[]" {
		t.Errorf("off board = %s", got)
	}
	s := g.Stack(sq("A1"))
	s[0].Kind = Capstone // callers get a copy
	if g.SquareString(sq("A1")) != "[F, w]" {
		t.Error("Stack must return a copy")
	}
}

func TestColor(t *testing.T) {
	if White.String() != "white" || Black.String() != "black" || White.Opponent() != Black || Black.Opponent() != White {
		t.Error("Color")
	}
}

func TestParseSquare(t *testing.T) {
	for in, want := range map[string]Square{"A1": {0, 0}, "e5": {4, 4}, "H8": {7, 7}, "A0": {0, -1}} {
		if got, ok := ParseSquare(in); !ok || got != want {
			t.Errorf("ParseSquare(%q) = %v, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "A", "A10", "11", "#1"} {
		if _, ok := ParseSquare(in); ok {
			t.Errorf("ParseSquare(%q) should fail", in)
		}
	}
	if (Square{File: 2, Rank: 4}).String() != "C5" {
		t.Error("Square.String")
	}
}

func TestTPS(t *testing.T) {
	g := New(std5)
	if got := g.TPS(); got != "x5/x5/x5/x5/x5 1 1" {
		t.Errorf("empty board %q", got)
	}
	play(t, g, "P A1", "P E5", "P C3 W")
	if got := g.TPS(); got != "x4,1/x5/x2,1S,x2/x5/2,x4 2 2" {
		t.Errorf("TPS %q", got)
	}
}
