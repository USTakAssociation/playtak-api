package tak

// Result is how a game ended, or None while it's still being played. The
// board decides roads, flat wins and draws; the game layer adds wins by
// resignation, time or abandonment, and aborts.
type Result int8

const (
	None Result = iota
	WhiteRoad
	BlackRoad
	WhiteFlat
	BlackFlat
	Draw
	WhiteWin // by resignation, time or abandonment
	BlackWin
	Aborted
)

// String is the result as sent to clients and stored in the games DB.
func (r Result) String() string {
	return [...]string{"---", "R-0", "0-R", "F-0", "0-F", "1/2-1/2", "1-0", "0-1", "0-0"}[r]
}

func roadWin(c Color) Result {
	if c == White {
		return WhiteRoad
	}
	return BlackRoad
}

// Win is the result for c winning by resignation, time or abandonment.
func Win(c Color) Result {
	if c == White {
		return WhiteWin
	}
	return BlackWin
}
