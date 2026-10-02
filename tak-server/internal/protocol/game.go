package protocol

// Game commands. Squares are written as on the wire, e.g. "A1".
type (
	PlaceStone struct {
		GameID   int
		Square   string
		Capstone bool
		Wall     bool
	}
	MoveStack struct {
		GameID   int
		From, To string
		Drops    []int
	}
	Resign     struct{ GameID int }
	ShowGame   struct{ GameID int }
	ShowSquare struct {
		GameID int
		Square string
	}
	ListGames struct{}

	// Undo and draw: the second player to ask agrees to the first's request.
	RequestUndo struct{ GameID int }
	RemoveUndo  struct{ GameID int }
	OfferDraw   struct{ GameID int }
	RemoveDraw  struct{ GameID int }
	// GiveTime adds a fixed amount to the opponent's clock.
	GiveTime struct{ GameID int }
)

func (PlaceStone) command()  {}
func (MoveStack) command()   {}
func (Resign) command()      {}
func (ShowGame) command()    {}
func (ShowSquare) command()  {}
func (ListGames) command()   {}
func (RequestUndo) command() {}
func (RemoveUndo) command()  {}
func (OfferDraw) command()   {}
func (RemoveDraw) command()  {}
func (GiveTime) command()    {}

// GameInfo describes a game in progress, as shown in the game list and at
// the start of a game.
type GameInfo struct {
	ID           int
	White, Black string
	Bot          bool // either player is a bot account
	GameSettings
}

// Game events.
type (
	// GameStarted tells a player their game has started, or resumed after
	// reconnecting. Color is the side they play.
	GameStarted struct {
		Game  GameInfo
		Color Color
	}
	// MovePlayed is a move in protocol notation, e.g. "P A1 C".
	MovePlayed struct {
		GameID int
		Move   string
	}
	// GameTime is both players' remaining time in milliseconds.
	GameTime struct {
		GameID       int
		White, Black int64
	}
	GameOver struct {
		GameID int
		Result string // e.g. "R-0", "1/2-1/2"
	}
	// GameAbandoned tells the remaining player their opponent didn't
	// reconnect in time.
	GameAbandoned struct {
		GameID  int
		Quitter string
	}
	// MoveRejected answers an illegal move with the reason.
	MoveRejected    struct{ Reason string }
	GameListAdded   struct{ Game GameInfo }
	GameListRemoved struct{ Game GameInfo }
	// PositionShown is the position in TPS.
	PositionShown struct {
		GameID int
		TPS    string
	}
	// SquareShown is one square's stack, e.g. "[F, f]".
	SquareShown struct {
		GameID int
		Stack  string
	}
	// UndoRequested and DrawOffered tell a player their opponent asked;
	// UndoRemoved and DrawRemoved that the request was withdrawn.
	UndoRequested struct{ GameID int }
	UndoRemoved   struct{ GameID int }
	DrawOffered   struct{ GameID int }
	DrawRemoved   struct{ GameID int }
	// Undone means both players agreed and the last ply was taken back.
	Undone struct{ GameID int }
	// TimeGiven means To was given Millis of extra time by their opponent.
	TimeGiven struct {
		GameID int
		To     Color
		Millis int64
	}
)

func (GameStarted) event()     {}
func (MovePlayed) event()      {}
func (GameTime) event()        {}
func (GameOver) event()        {}
func (GameAbandoned) event()   {}
func (MoveRejected) event()    {}
func (GameListAdded) event()   {}
func (GameListRemoved) event() {}
func (PositionShown) event()   {}
func (SquareShown) event()     {}
func (UndoRequested) event()   {}
func (UndoRemoved) event()     {}
func (DrawOffered) event()     {}
func (DrawRemoved) event()     {}
func (Undone) event()          {}
func (TimeGiven) event()       {}
