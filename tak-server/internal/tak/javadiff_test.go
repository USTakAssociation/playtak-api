//go:build javadiff

// Differential test against the Java server's game engine. It plays random
// games through both engines and checks after every move that they agree on
// whether the move was legal (and why not), every stack, both reserves, the
// move list and the result.
//
// It needs a JDK and the built Java server:
//
//	(cd server && mvn package)
//	go test -tags javadiff ./internal/tak/ -run JavaDiff -v
//
// TAK_JAVA_JAR overrides the jar path, TAK_DIFF_GAMES the number of games
// (default 500) and TAK_DIFF_SEED the random seed (default: time-based,
// printed so a failure can be replayed).
package tak

import (
	"bufio"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// javaReasons maps each Java error message to the Go messages that mean the
// same thing. Go splits some of Java's messages into more specific ones.
var javaReasons = map[string][]*regexp.Regexp{
	"Not your turn":                   {re(ErrNotYourTurn)},
	"First move should be flat":       {re(ErrFirstMoveFlat)},
	"Square not empty":                {re(ErrOccupied), re(ErrOffBoard)}, // Java says this for off-board placements too
	"You're out of capstones":         {re(ErrNoCapstones)},
	"First move should be place":      {re(ErrFirstMovePlace)},
	"Move should be in straight line": {re(ErrNotStraight)},
	"Out of bounds":                   {re(ErrOffBoard)},
	// Java's "do you control it" check treats an empty square as black's,
	// so white gets this message and black falls through to later checks.
	"You don't control stack": {re(ErrNotYourStack), re(ErrNoStack)},
	"Invalid move": {
		regexp.MustCompile(`^You can carry at most \d+ pieces$`),
		regexp.MustCompile(`^That stack only has \d+ pieces?$`),
		re(ErrNoStack),
	},
	"Invalid move.": {re(ErrDropCount), re(ErrNoStack)},
	"Should place atleast one tile in rest of the squares": {re(ErrEmptyDrop), re(ErrNoStack)},
	"Can't stack over capstones":                           {re(ErrOntoCapstone)},
	"Can't stack over walls":                               {re(ErrOntoWall)},
	"Capstone should be on top to flatten walls":           {re(ErrFlattenNeedsCap)},
	"undo refused":                                         nil, // matched against Go's Undo returning false
}

func re(err error) *regexp.Regexp {
	return regexp.MustCompile("^" + regexp.QuoteMeta(err.Error()) + "$")
}

var javaStates = map[Result]string{
	None: "NONE", WhiteRoad: "WHITE_ROAD", BlackRoad: "BLACK_ROAD",
	WhiteFlat: "WHITE_TILE", BlackFlat: "BLACK_TILE", Draw: "DRAW",
}

// standard piece counts by board size.
var standard = map[int][2]int{3: {10, 0}, 4: {15, 0}, 5: {21, 1}, 6: {30, 1}, 7: {40, 2}, 8: {50, 2}}

type javaEngine struct {
	in  io.Writer
	out *bufio.Scanner
}

func startJava(t *testing.T) *javaEngine {
	t.Helper()
	jar := os.Getenv("TAK_JAVA_JAR")
	if jar == "" {
		jar = filepath.Join("..", "..", "..", "server", "target", "takserver-jar-with-dependencies.jar")
	}
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("Java server jar not found at %s (build it with mvn package): %v", jar, err)
	}
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac not found")
	}
	classes := t.TempDir()
	if out, err := exec.Command("javac", "-cp", jar, "-d", classes, filepath.Join("testdata", "javadiff", "DiffHarness.java")).CombinedOutput(); err != nil {
		t.Fatalf("compile harness: %v\n%s", err, out)
	}
	cmd := exec.Command("java", "-cp", classes+string(os.PathListSeparator)+jar, "tak.DiffHarness")
	cmd.Stderr = io.Discard
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Wait()
	})
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	return &javaEngine{in: stdin, out: sc}
}

func (j *javaEngine) send(t *testing.T, cmd string, lines int) []string {
	t.Helper()
	fmt.Fprintln(j.in, cmd)
	out := make([]string, 0, lines)
	for range lines {
		if !j.out.Scan() {
			t.Fatalf("java harness exited after %q: %v", cmd, j.out.Err())
		}
		out = append(out, j.out.Text())
	}
	return out
}

// command is one action applied to both engines.
type command struct {
	undo     bool
	by       Color
	place    bool
	sq, to   Square
	kind     Kind
	drops    []int
	javaLine string
}

func (c command) String() string { return c.javaLine }

func placeCmd(by Color, sq Square, kind Kind) command {
	return command{by: by, place: true, sq: sq, kind: kind, javaLine: fmt.Sprintf("P %s %c %d %d %d",
		who(by), 'A'+sq.File, sq.Rank+1, b2i(kind == Capstone), b2i(kind == Wall))}
}

func moveCmd(by Color, from, to Square, drops []int) command {
	var ds []string
	for _, d := range drops {
		ds = append(ds, strconv.Itoa(d))
	}
	return command{by: by, sq: from, to: to, drops: drops, javaLine: fmt.Sprintf("M %s %c %d %c %d %s",
		who(by), 'A'+from.File, from.Rank+1, 'A'+to.File, to.Rank+1, strings.Join(ds, " "))}
}

func who(c Color) string {
	if c == White {
		return "w"
	}
	return "b"
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestJavaDiff(t *testing.T) {
	java := startJava(t)
	games := 500
	if n, err := strconv.Atoi(os.Getenv("TAK_DIFF_GAMES")); err == nil {
		games = n
	}
	seed := uint64(time.Now().UnixNano())
	if s, err := strconv.ParseUint(os.Getenv("TAK_DIFF_SEED"), 10, 64); err == nil {
		seed = s
	}
	t.Logf("seed %d (rerun with TAK_DIFF_SEED=%d), %d games", seed, seed, games)
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))

	stats := map[string]int{}
	for _, sg := range scriptedGames {
		t.Run(sg.name, func(t *testing.T) {
			g := New(sg.settings)
			java.send(t, fmt.Sprintf("NEW %d %d %d %d %d", sg.settings.Size, sg.settings.Pieces, sg.settings.Capstones, sg.settings.Komi, sg.settings.Opening), 1)
			var history []string
			for _, m := range sg.moves {
				c := scriptedCommand(g, m)
				history = append(history, c.javaLine)
				if !compareStep(t, java, g, c, sg.name, sg.settings, history, stats) {
					return
				}
			}
			if g.Result() != sg.want {
				t.Errorf("result %v, want %v", g.Result(), sg.want)
			}
		})
	}
	for i := range games {
		playDiffGame(t, java, rng, i, stats)
		if t.Failed() {
			return
		}
	}
	keys := slices.Sorted(maps.Keys(stats))
	for _, k := range keys {
		t.Logf("%6d  %s", stats[k], k)
	}
}

func playDiffGame(t *testing.T, java *javaEngine, rng *rand.Rand, n int, stats map[string]int) {
	size := 3 + rng.IntN(6)
	std := standard[size]
	s := Settings{
		Size:      size,
		Pieces:    max(1, std[0]-rng.IntN(std[0]/2+1)), // sometimes short, to reach out-of-pieces endings
		Capstones: rng.IntN(std[1] + 2),
		Komi:      rng.IntN(9),
	}
	if rng.IntN(3) == 0 {
		s.Opening = protocol.OpeningDoubleBlackStack
	}
	g := New(s)
	java.send(t, fmt.Sprintf("NEW %d %d %d %d %d", s.Size, s.Pieces, s.Capstones, s.Komi, s.Opening), 1)

	// In some games, stop placing after a while so the 50-ply rule comes up.
	shuffleFrom := -1
	if rng.IntN(4) == 0 {
		shuffleFrom = 10 + rng.IntN(30)
	}
	var history []string
	for ply := 0; ply < 600 && g.Result() == None; ply++ {
		var c command
		switch r := rng.Float64(); {
		case shuffleFrom >= 0 && ply >= shuffleFrom && r < 0.97:
			c = pickSpread(rng, g)
		case r < 0.04:
			c = command{undo: true, javaLine: "U"}
		case r < 0.14:
			c = randomCommand(rng, g)
		case r < 0.24:
			c = aimedMove(rng, g)
		default:
			legal := legalMoves(g)
			if len(legal) == 0 {
				t.Fatalf("game %d: no legal moves in %s", n, g.TPS())
			}
			c = legal[rng.IntN(len(legal))]
		}
		history = append(history, c.javaLine)
		if !compareStep(t, java, g, c, fmt.Sprintf("game %d", n), s, history, stats) {
			return
		}
	}
	stats["game result: "+g.Result().String()]++
	if g.Result() == Draw && len(history) > 0 && strings.HasPrefix(history[len(history)-1], "M") && !g.boardFull() {
		stats["situation: draw by 50-ply rule"]++
	}
}

// pickSpread returns a random legal spread, or any legal move if there's none.
func pickSpread(rng *rand.Rand, g *Game) command {
	legal := legalMoves(g)
	var moves []command
	for _, c := range legal {
		if !c.place {
			moves = append(moves, c)
		}
	}
	if len(moves) == 0 {
		moves = legal
	}
	return moves[rng.IntN(len(moves))]
}

var numbers = regexp.MustCompile(`\d+`)

func reasonMatches(javaMsg, goMsg string) bool {
	for _, re := range javaReasons[javaMsg] {
		if re.MatchString(goMsg) {
			return true
		}
	}
	return false
}

// randomCommand returns a command that is often illegal: random squares
// (including just off the board), kinds, drops and sometimes the wrong player.
func randomCommand(rng *rand.Rand, g *Game) command {
	by := g.Turn()
	if rng.IntN(8) == 0 {
		by = by.Opponent()
	}
	square := func() Square {
		// Files A..size and ranks 0..size+1 reach one square past each edge;
		// both stay within what the protocol's regex can express.
		return Square{File: rng.IntN(g.Size() + 1), Rank: rng.IntN(g.Size()+2) - 1}
	}
	if rng.IntN(2) == 0 {
		return placeCmd(by, square(), Kind(rng.IntN(3)))
	}
	from := square()
	to := from
	if rng.IntN(2) == 0 {
		to.File = rng.IntN(g.Size() + 1)
	} else {
		to.Rank = rng.IntN(g.Size()+2) - 1
	}
	if rng.IntN(10) == 0 {
		to = square() // possibly not in a straight line
	}
	drops := make([]int, 1+rng.IntN(3))
	for i := range drops {
		drops[i] = rng.IntN(min(g.Size()+2, 10)) // includes 0 and too-large values
	}
	return moveCmd(by, from, to, drops)
}

// aimedMove moves one of the mover's real stacks in a real direction with
// random drops, so it often runs into walls, capstones or the board edge.
func aimedMove(rng *rand.Rand, g *Game) command {
	by := g.Turn()
	var mine []Square
	for r := range g.Size() {
		for f := range g.Size() {
			sq := Square{File: f, Rank: r}
			if s := g.Stack(sq); len(s) > 0 && s[len(s)-1].Color == by {
				mine = append(mine, sq)
			}
		}
	}
	if len(mine) == 0 {
		return randomCommand(rng, g)
	}
	from := mine[rng.IntN(len(mine))]
	d := [...]Square{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}[rng.IntN(4)]
	drops := make([]int, 1+rng.IntN(min(4, g.Size())))
	for i := range drops {
		drops[i] = 1 + rng.IntN(2)
	}
	to := Square{File: from.File + d.File*len(drops), Rank: from.Rank + d.Rank*len(drops)}
	if to.File < 0 || to.File > 25 || to.Rank < -1 || to.Rank > 8 {
		return randomCommand(rng, g) // beyond what the protocol can express
	}
	return moveCmd(by, from, to, drops)
}

// legalMoves lists every legal move for the player to move.
func legalMoves(g *Game) []command {
	by := g.Turn()
	stones, caps := g.Reserves(by)
	opening := g.Ply() < 2
	var out []command
	for r := range g.Size() {
		for f := range g.Size() {
			sq := Square{File: f, Rank: r}
			stack := g.Stack(sq)
			if len(stack) == 0 {
				if stones > 0 {
					out = append(out, placeCmd(by, sq, Flat))
					if !opening {
						out = append(out, placeCmd(by, sq, Wall))
					}
				}
				if caps > 0 && !opening {
					out = append(out, placeCmd(by, sq, Capstone))
				}
				continue
			}
			if opening || stack[len(stack)-1].Color != by {
				continue
			}
			top := stack[len(stack)-1]
			for _, d := range [...]Square{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				for carry := 1; carry <= min(g.Size(), len(stack)); carry++ {
					spreads(g, sq, d, carry, top.Kind == Capstone, nil, func(drops []int) {
						to := Square{File: sq.File + d.File*len(drops), Rank: sq.Rank + d.Rank*len(drops)}
						out = append(out, moveCmd(by, sq, to, append([]int(nil), drops...)))
					})
				}
			}
		}
	}
	return out
}

// spreads calls emit for every way to drop the remaining stones moving in
// direction d from the square after `from` plus len(drops) steps.
func spreads(g *Game, from, d Square, remaining int, capOnTop bool, drops []int, emit func([]int)) {
	next := Square{File: from.File + d.File*(len(drops)+1), Rank: from.Rank + d.Rank*(len(drops)+1)}
	if !g.onBoard(next) {
		return
	}
	stack := g.Stack(next)
	if len(stack) > 0 {
		switch stack[len(stack)-1].Kind {
		case Capstone:
			return
		case Wall:
			// Only a lone capstone can finish here, flattening the wall.
			if remaining == 1 && capOnTop {
				emit(append(drops, 1))
			}
			return
		}
	}
	for drop := 1; drop <= remaining; drop++ {
		if drop == remaining {
			emit(append(drops, drop))
		} else {
			spreads(g, from, d, remaining-drop, capOnTop, append(drops, drop), emit)
		}
	}
}

// compareStep applies c to the Go engine and the Java harness and checks
// they agree. It returns false if the game should stop: on a mismatch
// (reported as a test error) or when the engines have intentionally diverged.
func compareStep(t *testing.T, java *javaEngine, g *Game, c command, label string, s Settings, history []string, stats map[string]int) bool {
	t.Helper()
	var goErr error
	goOK := true
	switch {
	case c.undo:
		goOK = g.Undo()
	case c.place:
		goErr = g.Place(c.by, c.sq, c.kind)
	default:
		goErr = g.Move(c.by, c.sq, c.to, c.drops)
	}
	if goErr != nil {
		goOK = false
	}

	out := java.send(t, c.javaLine, 4)
	javaOK := out[0] == "OK"
	if goErr == ErrNoStones && javaOK {
		// Intentional fix: Java lets a player with no stones left place one,
		// driving their reserve negative. The engines have diverged.
		stats["game skipped: java negative reserve bug"]++
		return false
	}
	fail := func(format string, args ...any) bool {
		t.Errorf("%s (%+v) after %q:\n  %s\nmoves:\n  %s",
			label, s, c, fmt.Sprintf(format, args...), strings.Join(history, "\n  "))
		return false
	}
	if goOK != javaOK {
		return fail("go ok=%v err=%v, java %q", goOK, goErr, out[0])
	}
	if goErr != nil {
		javaMsg := strings.TrimPrefix(out[0], "ERR ")
		if !reasonMatches(javaMsg, goErr.Error()) {
			return fail("go rejected with %q, java with %q", goErr, javaMsg)
		}
		stats["rejected: "+numbers.ReplaceAllString(goErr.Error(), "N")]++
	}
	if c.undo {
		stats[fmt.Sprintf("undo accepted=%v", goOK)]++
	}
	if goOK && !c.undo && g.hasRoad(White) && g.hasRoad(Black) {
		stats["situation: both roads completed at once"]++
	}

	wS, wC := g.Reserves(White)
	bS, bC := g.Reserves(Black)
	wantState := fmt.Sprintf("STATE %s %d %d %d %d %d", javaStates[g.Result()], g.Ply(), wS, wC, bS, bC)
	if out[1] != wantState {
		return fail("go %s, java %s", wantState, out[1])
	}
	var squares []string
	for r := range g.Size() {
		for f := range g.Size() {
			squares = append(squares, g.SquareString(Square{File: f, Rank: r}))
		}
	}
	if wantBoard := "BOARD " + strings.Join(squares, "|"); out[2] != wantBoard {
		return fail("boards differ\n  go   %s\n  java %s", wantBoard, out[2])
	}
	if wantMoves := "MOVES " + strings.Join(g.Moves(), ","); out[3] != wantMoves {
		return fail("move lists differ\n  go   %s\n  java %s", wantMoves, out[3])
	}
	return true
}

// scriptedGames reach situations random play rarely does.
var scriptedGames = []struct {
	name     string
	settings Settings
	moves    []string // each played by the player to move
	want     Result
}{
	{
		// Black lifts its stone off A5 onto A4: that uncovers white's road on
		// rank 5 and completes black's on rank 4. The mover wins.
		name:     "double road, black moves",
		settings: std5,
		moves: []string{
			"P B4", "P B5", "P C5", "P C4", "P D5", "P D4", "P A5", "P A4",
			"P A1", "M A4 A5 1", "P E5", "P E4", "P A2", "M A5 A4 1",
		},
		want: BlackRoad,
	},
	{
		name:     "double road, white moves",
		settings: std5,
		moves: []string{
			"P B5", "P B4", "P C4", "P C5", "P D4", "P D5", "P A4", "P A5",
			"M A4 A5 1", "P E5", "P E4", "P A1", "M A5 A4 1",
		},
		want: WhiteRoad,
	},
}

func scriptedCommand(g *Game, move string) command {
	f := strings.Fields(move)
	if f[0] == "P" {
		kind := Flat
		if len(f) > 2 {
			kind = map[string]Kind{"W": Wall, "C": Capstone}[f[2]]
		}
		return placeCmd(g.Turn(), sq(f[1]), kind)
	}
	var drops []int
	for _, d := range f[3:] {
		n, _ := strconv.Atoi(d)
		drops = append(drops, n)
	}
	return moveCmd(g.Turn(), sq(f[1]), sq(f[2]), drops)
}
