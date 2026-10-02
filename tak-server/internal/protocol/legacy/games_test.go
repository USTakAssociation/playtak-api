package legacy

import (
	"reflect"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func TestParseGameCommands(t *testing.T) {
	for _, tc := range []struct {
		line string
		want protocol.Command
	}{
		{"Game#12 P A1", protocol.PlaceStone{GameID: 12, Square: "A1"}},
		{"Game#12 P C3 C", protocol.PlaceStone{GameID: 12, Square: "C3", Capstone: true}},
		{"Game#12 P C3 W", protocol.PlaceStone{GameID: 12, Square: "C3", Wall: true}},
		{"Game#12 P C3 C W", protocol.PlaceStone{GameID: 12, Square: "C3", Capstone: true}},
		{"Game#12 P Z9", protocol.PlaceStone{GameID: 12, Square: "Z9"}}, // range is checked by the game
		{"Game#12 P a1", nil},
		{"Game#12 M A1 A3 1 2", protocol.MoveStack{GameID: 12, From: "A1", To: "A3", Drops: []int{1, 2}}},
		{"Game#12 M A1 A2 1", protocol.MoveStack{GameID: 12, From: "A1", To: "A2", Drops: []int{1}}},
		{"Game#12 M A1 A2", nil}, // drops are required
		{"Game#12 Resign", protocol.Resign{GameID: 12}},
		{"Game#12 Show", protocol.ShowGame{GameID: 12}},
		{"Game#12 Show A1", protocol.ShowSquare{GameID: 12, Square: "A1"}},
		{"Game#12 Show x", nil},
		{"GameList", protocol.ListGames{}},
		{"Game#12 RequestUndo", protocol.RequestUndo{GameID: 12}},
		{"Game#12 RemoveUndo", protocol.RemoveUndo{GameID: 12}},
		{"Game#12 OfferDraw", protocol.OfferDraw{GameID: 12}},
		{"Game#12 RemoveDraw", protocol.RemoveDraw{GameID: 12}},
		{"Game#12 GiveTime", protocol.GiveTime{GameID: 12}},
		{"Game#12 GiveTime 5", nil}, // anchored in Java, unlike the others
		{"Game#12 OfferDraw please", protocol.OfferDraw{GameID: 12}},
		{"Game#99999999999999999999 OfferDraw", nil},
		{"Game# OfferDraw", nil},
		{"Game#99999999999999999999 P A1", nil},
	} {
		if got := Parse(tc.line, true); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q) = %#v, want %#v", tc.line, got, tc.want)
		}
	}
	if Parse("Game#1 P A1", false) != nil {
		t.Error("game commands require login")
	}
}

var info = protocol.GameInfo{
	ID: 12, White: "alice", Black: "bobby", Bot: true,
	GameSettings: protocol.GameSettings{
		Size: 5, Time: 600, Increment: 30, IncrementScales: true, Komi: 4, Pieces: 21, Capstones: 1,
		Tournament: true, ExtraTimeTrigger: 20, ExtraTimeAmount: 60, Opening: protocol.OpeningDoubleBlackStack,
	},
}

func TestFormatGameEvents(t *testing.T) {
	for _, tc := range []struct {
		ev      protocol.Event
		version int
		want    []string
	}{
		{protocol.GameStarted{Game: info, Color: protocol.White}, 0, []string{"Game Start 12 5 alice vs bobby white 600 4 21 1 20 60"}},
		{protocol.GameStarted{Game: info, Color: protocol.Black}, 1, []string{"Game Start 12 5 alice vs bobby black 600 4 21 1 20 60"}},
		{protocol.GameStarted{Game: info, Color: protocol.White}, 2, []string{"Game Start 12 alice vs bobby white 5 600 30 4 21 1 0 1 20 60 1"}},
		{protocol.GameStarted{Game: info, Color: protocol.Black}, 4, []string{"Game Start 12 alice vs bobby black 5 600 30 1 4 21 1 0 1 20 60 1 1"}},
		{protocol.MovePlayed{GameID: 12, Move: "M A1 A3 1 2"}, 0, []string{"Game#12 M A1 A3 1 2"}},
		{protocol.GameTime{GameID: 12, White: 599_999, Black: -5}, 0, []string{"Game#12 Time 599 0"}},
		{protocol.GameTime{GameID: 12, White: 599_999, Black: -5}, 1, []string{"Game#12 Timems 599999 0"}},
		{protocol.GameOver{GameID: 12, Result: "R-0"}, 0, []string{"Game#12 Over R-0"}},
		{protocol.GameAbandoned{GameID: 12, Quitter: "bobby"}, 0, []string{"Game#12 Abandoned. bobby quit"}},
		{protocol.MoveRejected{Reason: "Not your turn"}, 0, []string{"NOK", "Error:Not your turn"}},
		{protocol.GameListAdded{Game: info}, 3, []string{"GameList Add 12 alice bobby 5 600 30 4 21 1 0 1 20 60"}},
		{protocol.GameListRemoved{Game: info}, 4, []string{"GameList Remove 12 alice bobby 5 600 30 1 4 21 1 0 1 20 60 1"}},
		{protocol.PositionShown{GameID: 12, TPS: "x5/x5/x5/x5/x5 1 1"}, 0, []string{"Game#12 Show x5/x5/x5/x5/x5 1 1"}},
		{protocol.SquareShown{GameID: 12, Stack: "[F, f]"}, 0, []string{"Game#12 Show Sq [F, f]"}},
		{protocol.UndoRequested{GameID: 12}, 0, []string{"Game#12 RequestUndo"}},
		{protocol.UndoRemoved{GameID: 12}, 0, []string{"Game#12 RemoveUndo"}},
		{protocol.Undone{GameID: 12}, 0, []string{"Game#12 Undo"}},
		{protocol.DrawOffered{GameID: 12}, 0, []string{"Game#12 OfferDraw"}},
		{protocol.DrawRemoved{GameID: 12}, 0, []string{"Game#12 RemoveDraw"}},
		{protocol.TimeGiven{GameID: 12, To: protocol.White, Millis: 15_000}, 0, []string{"Game#12 GivenTime white 15000"}},
		{protocol.TimeGiven{GameID: 12, To: protocol.Black, Millis: 15_000}, 4, []string{"Game#12 GivenTime black 15000"}},
	} {
		got, err := Format(tc.ev, tc.version)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Format(%T, v%d) = %q, %v; want %q", tc.ev, tc.version, got, err, tc.want)
		}
	}
}
