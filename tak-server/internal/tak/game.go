package tak

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Errors for rejected moves. Their text is shown to players.
var (
	ErrGameOver        = errors.New("The game is over")
	ErrNotYourTurn     = errors.New("Not your turn")
	ErrOffBoard        = errors.New("That square is off the board")
	ErrOccupied        = errors.New("That square is not empty")
	ErrFirstMoveFlat   = errors.New("The first move must place a flat")
	ErrNoCapstones     = errors.New("You have no capstones left")
	ErrNoStones        = errors.New("You have no stones left")
	ErrFirstMovePlace  = errors.New("The first move must be a placement")
	ErrNotStraight     = errors.New("Moves must be in a straight line")
	ErrNoStack         = errors.New("There is no stack on that square")
	ErrNotYourStack    = errors.New("You don't control that stack")
	ErrDropCount       = errors.New("The number of drops must match the distance moved")
	ErrEmptyDrop       = errors.New("Each square moved over must get at least one piece")
	ErrNoDistance      = errors.New("A move must leave its square")
	ErrOntoCapstone    = errors.New("Can't move onto a capstone")
	ErrOntoWall        = errors.New("Can't move onto a wall")
	ErrFlattenNeedsCap = errors.New("Only a capstone moving alone can flatten a wall")
)

// staleLimit is how many plies may pass without a placement or a flattened
// wall before the game is drawn.
const staleLimit = 50

const defaultSize = 5

// Settings are the rules a game is played with. Komi is in half-flats and
// is added to black's score.
type Settings struct {
	Size      int
	Pieces    int // stones per player, excluding capstones
	Capstones int // per player
	Komi      int
	Opening   protocol.Opening
}

// position is everything that changes from ply to ply, so undo can restore
// it whole.
type position struct {
	stacks    [][]Stone // indexed by rank*size + file, bottom stone first
	stones    [2]int    // reserve stones per color
	caps      [2]int    // reserve capstones per color
	ply       int       // plies played
	lastReset int       // ply of the last placement or flattened wall
}

func (p position) clone() position {
	c := p
	c.stacks = make([][]Stone, len(p.stacks))
	for i, s := range p.stacks {
		c.stacks[i] = append([]Stone(nil), s...)
	}
	return c
}

// Game is a game of Tak in progress. It isn't safe for concurrent use.
type Game struct {
	size     int
	komi     int
	opening  protocol.Opening
	position position
	history  []position // the position before each ply, for undo
	moves    []string
	result   Result
}

// New starts a game. A size outside 3-8 falls back to 5.
func New(settings Settings) *Game {
	size := settings.Size
	if size < 3 || size > 8 {
		size = defaultSize
	}
	return &Game{
		size:    size,
		komi:    settings.Komi,
		opening: settings.Opening,
		position: position{
			stacks: make([][]Stone, size*size),
			stones: [2]int{settings.Pieces, settings.Pieces},
			caps:   [2]int{settings.Capstones, settings.Capstones},
		},
	}
}

func (g *Game) Size() int { return g.size }

// Ply is the number of plies played.
func (g *Game) Ply() int { return g.position.ply }

// Turn is the color to move.
func (g *Game) Turn() Color { return Color(g.position.ply % 2) }

// Result is the result decided by the position, or None while it's open.
func (g *Game) Result() Result { return g.result }

// Reserves returns how many stones and capstones c has left to place.
func (g *Game) Reserves(c Color) (stones, capstones int) {
	return g.position.stones[c], g.position.caps[c]
}

// Moves returns the moves played, in protocol notation ("P A1 C",
// "M A1 A3 1 2").
func (g *Game) Moves() []string { return append([]string(nil), g.moves...) }

// Stack returns the stones on square, bottom first, or nil if square is off the board.
func (g *Game) Stack(square Square) []Stone {
	if !g.onBoard(square) {
		return nil
	}
	return append([]Stone(nil), g.position.stacks[g.index(square)]...)
}

// Place puts a stone of kind on square for player by. During each player's first
// ply the stone placed is the opponent's, and must be a flat.
func (g *Game) Place(by Color, square Square, kind Kind) error {
	if g.result != None {
		return ErrGameOver
	}
	if by != g.Turn() {
		return ErrNotYourTurn
	}
	if !g.onBoard(square) {
		return ErrOffBoard
	}
	if len(g.position.stacks[g.index(square)]) > 0 {
		return ErrOccupied
	}
	opening := g.position.ply < 2
	if opening && kind != Flat {
		return ErrFirstMoveFlat
	}
	if kind == Capstone && g.position.caps[by] == 0 {
		return ErrNoCapstones
	}
	if kind != Capstone && g.position.stones[by] == 0 {
		return ErrNoStones
	}

	g.history = append(g.history, g.position.clone())
	owner := by
	if opening {
		owner = by.Opponent()
	}
	// The stone comes from the mover's reserve even when it's the opponent's
	// color, so both reserves drop by one over the opening.
	if kind == Capstone {
		g.position.caps[by]--
	} else {
		g.position.stones[by]--
	}
	i := g.index(square)
	g.position.stacks[i] = append(g.position.stacks[i], Stone{Color: owner, Kind: kind})
	// Double black stack: white's first placement is two black flats, the
	// second taken from black's reserve.
	if g.opening == protocol.OpeningDoubleBlackStack && g.position.ply == 0 {
		g.position.stones[Black]--
		g.position.stacks[i] = append(g.position.stacks[i], Stone{Color: Black, Kind: Flat})
	}

	move := "P " + square.String()
	switch kind {
	case Wall:
		move += " W"
	case Capstone:
		move += " C"
	}
	g.moves = append(g.moves, move)
	g.position.ply++
	g.position.lastReset = g.position.ply
	g.decide(true)
	return nil
}

// Move picks up stones from the top of the stack on from and drops them
// one square at a time towards to, drops[i] stones on the (i+1)th square.
func (g *Game) Move(by Color, from, to Square, drops []int) error {
	if g.result != None {
		return ErrGameOver
	}
	if by != g.Turn() {
		return ErrNotYourTurn
	}
	if g.position.ply < 2 {
		return ErrFirstMovePlace
	}
	if from.File != to.File && from.Rank != to.Rank {
		return ErrNotStraight
	}
	if !g.onBoard(from) || !g.onBoard(to) {
		return ErrOffBoard
	}
	src := g.position.stacks[g.index(from)]
	if len(src) == 0 {
		return ErrNoStack
	}
	top := src[len(src)-1]
	if top.Color != by {
		return ErrNotYourStack
	}
	carry := 0
	for _, d := range drops {
		// One drop over the size already breaks the carry limit; checking it
		// first also keeps huge values from overflowing the sum.
		if d > g.size {
			return fmt.Errorf("You can carry at most %d pieces", g.size)
		}
		carry += d
	}
	if carry > g.size {
		return fmt.Errorf("You can carry at most %d pieces", g.size)
	}
	if carry > len(src) {
		return fmt.Errorf("That stack only has %s", pieces(len(src)))
	}
	dist := max(abs(to.File-from.File), abs(to.Rank-from.Rank))
	if len(drops) != dist {
		return ErrDropCount
	}
	// After the drop count check, so a same-square move with drops gets the
	// same message as from the Java server.
	if dist == 0 {
		return ErrNoDistance
	}
	for _, d := range drops {
		if d < 1 {
			return ErrEmptyDrop
		}
	}
	step := Square{File: sign(to.File - from.File), Rank: sign(to.Rank - from.Rank)}
	for i := 1; i <= dist; i++ {
		s := g.position.stacks[g.index(Square{File: from.File + step.File*i, Rank: from.Rank + step.Rank*i})]
		if len(s) == 0 {
			continue
		}
		switch s[len(s)-1].Kind {
		case Capstone:
			return ErrOntoCapstone
		case Wall:
			if i != dist {
				return ErrOntoWall
			}
			if drops[len(drops)-1] != 1 || top.Kind != Capstone {
				return ErrFlattenNeedsCap
			}
		}
	}

	g.history = append(g.history, g.position.clone())
	fi := g.index(from)
	carried := append([]Stone(nil), src[len(src)-carry:]...)
	g.position.stacks[fi] = src[:len(src)-carry]
	for i, drop := range drops {
		di := g.index(Square{File: from.File + step.File*(i+1), Rank: from.Rank + step.Rank*(i+1)})
		dst := g.position.stacks[di]
		if len(dst) > 0 && dst[len(dst)-1].Kind == Wall {
			// Only reachable on the last square with a lone capstone.
			dst[len(dst)-1].Kind = Flat
			g.position.lastReset = g.position.ply + 1
		}
		g.position.stacks[di] = append(dst, carried[:drop]...)
		carried = carried[drop:]
	}

	move := fmt.Sprintf("M %s %s", from, to)
	for _, d := range drops {
		move += " " + strconv.Itoa(d)
	}
	g.moves = append(g.moves, move)
	g.position.ply++
	g.decide(false)
	return nil
}

// Undo takes back the last ply. It reports false if there is nothing to
// undo or the game is already decided.
func (g *Game) Undo() bool {
	if len(g.history) == 0 || g.result != None {
		return false
	}
	g.position = g.history[len(g.history)-1]
	g.history = g.history[:len(g.history)-1]
	g.moves = g.moves[:len(g.moves)-1]
	return true
}

// decide sets the result if the ply just played ended the game. A road beats
// running out of pieces or squares; if both players complete a road at once,
// the player who moved wins.
func (g *Game) decide(placement bool) {
	white, black := g.hasRoad(White), g.hasRoad(Black)
	mover := g.Turn().Opponent()
	switch {
	case white && black:
		g.result = roadWin(mover)
	case white:
		g.result = WhiteRoad
	case black:
		g.result = BlackRoad
	}
	if g.result != None {
		return
	}
	outOfPieces := func(c Color) bool { return g.position.stones[c] == 0 && g.position.caps[c] == 0 }
	if placement && (outOfPieces(White) || outOfPieces(Black)) || g.boardFull() {
		g.result = g.flatResult()
		return
	}
	if !placement && g.position.lastReset+staleLimit <= g.position.ply {
		g.result = Draw
	}
}

// flatResult counts flats on top of stacks, with komi added to black.
func (g *Game) flatResult() Result {
	var score [2]int
	for _, s := range g.position.stacks {
		if len(s) > 0 && s[len(s)-1].Kind == Flat {
			score[s[len(s)-1].Color] += 2
		}
	}
	score[Black] += g.komi
	switch {
	case score[White] > score[Black]:
		return WhiteFlat
	case score[Black] > score[White]:
		return BlackFlat
	}
	return Draw
}

func (g *Game) boardFull() bool {
	for _, s := range g.position.stacks {
		if len(s) == 0 {
			return false
		}
	}
	return true
}

func (g *Game) onBoard(square Square) bool {
	return square.File >= 0 && square.File < g.size && square.Rank >= 0 && square.Rank < g.size
}

func (g *Game) index(square Square) int { return square.Rank*g.size + square.File }

// SquareString shows a square's stack bottom first, e.g. "[F, f, C]", or "[]"
// for an empty or off-board square.
func (g *Game) SquareString(square Square) string {
	var parts []string
	for _, s := range g.Stack(square) {
		parts = append(parts, string(s.letter()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func pieces(n int) string {
	if n == 1 {
		return "1 piece"
	}
	return strconv.Itoa(n) + " pieces"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}
