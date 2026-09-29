package protocol

// Color is the side a seek's creator will play.
type Color int

const (
	AnyColor Color = iota
	White
	Black
)

// Opening is the first-move variant.
type Opening int

const (
	OpeningSwap             Opening = 0
	OpeningDoubleBlackStack Opening = 1
)

// PTN returns the opening's PTN Ninja tag value, as stored in the games DB.
func (o Opening) PTN() string {
	if o == OpeningDoubleBlackStack {
		return "double black stack"
	}
	return "swap"
}

// GameSettings are the rules and clock a seek offers and a game is played with.
// Times are in seconds; Komi is in half-flats.
type GameSettings struct {
	Size             int
	Time             int
	Increment        int
	IncrementScales  bool // increment multiplied by the mover's move number
	Color            Color
	Komi             int
	Pieces           int
	Capstones        int
	Unrated          bool
	Tournament       bool
	ExtraTimeTrigger int // move number at which ExtraTimeAmount is added
	ExtraTimeAmount  int
	Opening          Opening
}

// RequiredProtocolVersion is the lowest protocol version that can play,
// watch or list a game with these settings without its board going out of
// sync with the server's.
//
// Only settings that change how moves map onto the board count. A
// non-swap opening does (its first move looks like an ordinary placement,
// so a client that isn't told about it renders the wrong position). Clock
// settings such as IncrementScales don't: the server sends absolute clock
// values after every move, so an older client corrects itself.
func (g GameSettings) RequiredProtocolVersion() int {
	if g.Opening != OpeningSwap {
		return 4
	}
	return 0
}

// Seek is an open offer to play, as shown to clients.
type Seek struct {
	ID       int
	Creator  string
	Bot      bool   // the creator is a bot account
	Opponent string // only this player may accept; empty for anyone
	GameSettings
}
