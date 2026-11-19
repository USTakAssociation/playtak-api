package legacy

import (
	"reflect"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func TestParseSeekCommands(t *testing.T) {
	g := func(mod func(*protocol.GameSettings)) protocol.GameSettings {
		s := protocol.GameSettings{Size: 5, Time: 600, Increment: 30, Pieces: 21, Capstones: 1}
		if mod != nil {
			mod(&s)
		}
		return s
	}
	for _, tc := range []struct {
		line string
		want protocol.Command
	}{
		{"List", protocol.ListSeeks{}},
		{"Listen", protocol.ListSeeks{}}, // prefix match, as in Java
		{"Seek 5 600 30", protocol.PostSeek{Settings: g(nil)}},
		{"Seek 5 600 30 W", protocol.PostSeek{Settings: g(func(s *protocol.GameSettings) { s.Color = protocol.White })}},
		{"Seek 3 60 0", protocol.PostSeek{Settings: protocol.GameSettings{Size: 3, Time: 60, Pieces: 10}}},
		{"Seek 9 60 0", protocol.PostSeek{Settings: protocol.GameSettings{Size: 9, Time: 60}}}, // clamped later
		{"Seek 0 0 0", protocol.CancelSeek{}},
		{"Seek 5 600 30 A 4 21 1 1 0 ", protocol.PostSeek{Settings: g(func(s *protocol.GameSettings) {
			s.Komi, s.Unrated = 4, true
		})}},
		{"Seek 5 600 30 B 0 21 1 0 1 bob", protocol.PostSeek{Opponent: "bob", Settings: g(func(s *protocol.GameSettings) {
			s.Color, s.Tournament = protocol.Black, true
		})}},
		{"Seek 5 600 30 A 0 21 1 0 0 20 60 ", protocol.PostSeek{Settings: g(func(s *protocol.GameSettings) {
			s.ExtraTimeTrigger, s.ExtraTimeAmount = 20, 60
		})}},
		{"Seek 5 600 30 1 A 0 21 1 0 0 0 0 1 bob", protocol.PostSeek{Opponent: "bob", Settings: g(func(s *protocol.GameSettings) {
			s.IncrementScales, s.Opening = true, protocol.OpeningDoubleBlackStack
		})}},
		{"Seek 0 600 30 1 A 0 21 1 0 0 0 0 1 ", protocol.CancelSeek{}},
		{"Seek 5 99999999999999999999 30", nil}, // too large: NOK instead of a crash
		{"Accept 12", protocol.AcceptSeek{ID: 12}},
		{"Rematch 7 5 600 30 A 0 21 1 0 0 0 0 bob", protocol.Rematch{GameID: 7, Opponent: "bob", Settings: g(nil)}},
		{"Rematch 7 5 600 30 1 A 0 21 1 0 0 0 0 1 bob", protocol.Rematch{GameID: 7, Opponent: "bob", Settings: g(func(s *protocol.GameSettings) {
			s.IncrementScales, s.Opening = true, protocol.OpeningDoubleBlackStack
		})}},
	} {
		if got := Parse(tc.line, true); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q) = %#v, want %#v", tc.line, got, tc.want)
		}
	}
	if got := Parse("Seek 5 600 30", false); got != nil {
		t.Errorf("seeks require login, got %#v", got)
	}
}

func TestFormatSeek(t *testing.T) {
	s := protocol.Seek{ID: 3, Creator: "alice", GameSettings: protocol.GameSettings{
		Size: 5, Time: 600, Increment: 30, Color: protocol.White, Pieces: 21, Capstones: 1,
		ExtraTimeTrigger: 20, ExtraTimeAmount: 60,
	}}
	withOpp := s
	withOpp.Opponent, withOpp.Bot, withOpp.IncrementScales, withOpp.Opening = "bob", true, true, protocol.OpeningDoubleBlackStack

	for _, tc := range []struct {
		ev      protocol.Event
		version int
		want    string
	}{
		{protocol.SeekAdded{Seek: s}, 0, "Seek new 3 alice 5 600 30 W 0 21 1 0 0 20 60 "},
		{protocol.SeekAdded{Seek: withOpp}, 1, "Seek new 3 alice 5 600 30 W 0 21 1 0 0 20 60 bob"},
		{protocol.SeekAdded{Seek: s}, 2, "Seek new 3 alice 5 600 30 W 0 21 1 0 0 20 60 0 0"},
		{protocol.SeekRemoved{Seek: withOpp}, 3, "Seek remove 3 alice 5 600 30 W 0 21 1 0 0 20 60 bob 1"},
		{protocol.SeekAdded{Seek: s}, 4, "Seek new 3 alice 5 600 30 0 W 0 21 1 0 0 20 60 0 0 0"},
		{protocol.SeekAdded{Seek: withOpp}, 4, "Seek new 3 alice 5 600 30 1 W 0 21 1 0 0 20 60 bob 1 1"},
		{protocol.AcceptRematch{SeekID: 9}, 0, "Accept Rematch 9"},
		{protocol.RematchSeekCreated{SeekID: 9}, 0, "Rematch seek created with ID: 9"},
	} {
		got, err := Format(tc.ev, tc.version)
		if err != nil || len(got) != 1 || got[0] != tc.want {
			t.Errorf("Format(%T, v%d) = %q, %v; want %q", tc.ev, tc.version, got, err, tc.want)
		}
	}
}
