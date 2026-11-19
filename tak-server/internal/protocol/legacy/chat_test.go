package legacy

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func TestParseChatCommands(t *testing.T) {
	long := strings.Repeat("a", 300)
	emoji := strings.Repeat("😀", 300)
	room64 := strings.Repeat("r", 64)
	for _, tc := range []struct {
		line string
		want protocol.Command
	}{
		{"Shout hello world", protocol.PostShout{Text: "hello world"}},
		{"Shout  leading space", protocol.PostShout{Text: " leading space"}},
		{"Shout 😀 emoji", protocol.PostShout{Text: "😀 emoji"}},
		{"Shout " + long, protocol.PostShout{Text: long[:256]}},                   // cut, as Java's pattern
		{"Shout " + emoji, protocol.PostShout{Text: string([]rune(emoji)[:256])}}, // counted in characters
		{"Shout ", nil},
		{"Shout", nil},
		{"ShoutRoom lobby hi all", protocol.PostRoomShout{Room: "lobby", Text: "hi all"}},
		{"ShoutRoom " + room64 + " hi", protocol.PostRoomShout{Room: room64, Text: "hi"}},
		{"ShoutRoom " + room64 + "x hi", nil}, // a room name over 64 doesn't match
		{"ShoutRoom lobby", nil},
		{"ShoutRoom lobby ", nil},
		{"JoinRoom lobby", protocol.JoinRoom{Room: "lobby"}},
		{"JoinRoom " + room64 + "xyz", protocol.JoinRoom{Room: room64}}, // cut, as Java
		{"JoinRoom ", nil},
		{"LeaveRoom lobby", protocol.LeaveRoom{Room: "lobby"}},
		{"LeaveRoom", nil},
		{"Tell alice hi there", protocol.SendTell{To: "alice", Text: "hi there"}},
		{"Tell Guest12 hi", protocol.SendTell{To: "Guest12", Text: "hi"}},
		{"Tell bob hi", nil},    // names are at least 4 characters
		{"Tell 1alice hi", nil}, // and start with a letter
		{"Tell alice", nil},     // no message
		{"Tell al-ice hi", nil}, // invalid name character
	} {
		if got := Parse(tc.line, true); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q) = %#v, want %#v", tc.line, got, tc.want)
		}
	}
	for _, line := range []string{"Shout hi", "Tell alice hi", "JoinRoom x", "LeaveRoom x", "ShoutRoom x hi"} {
		if Parse(line, false) != nil {
			t.Errorf("%q should require login", line)
		}
	}
}

func TestFormatChatEvents(t *testing.T) {
	for _, tc := range []struct {
		ev   protocol.Event
		want []string
	}{
		{protocol.Shout{From: "alice", Text: "hi 😀"}, []string{"Shout <alice> hi 😀"}},
		{protocol.RoomJoined{Room: "lobby"}, []string{"Joined room lobby"}},
		{protocol.RoomShout{Room: "lobby", From: "alice", Text: "hi"}, []string{"ShoutRoom lobby <alice> hi"}},
		{protocol.TellReceived{From: "alice", Text: "psst"}, []string{"Tell <alice> psst"}},
		{protocol.Told{To: "bobby", Text: "psst"}, []string{"Told <bobby> psst"}},
		{protocol.CommandFailed{Reason: "Player is not online"}, []string{"Error:Player is not online"}},
		{protocol.RawLine{Text: "Error: anything at all"}, []string{"Error: anything at all"}},
	} {
		got, err := Format(tc.ev, 0)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("Format(%#v) = %q, %v; want %q", tc.ev, got, err, tc.want)
		}
	}
}
