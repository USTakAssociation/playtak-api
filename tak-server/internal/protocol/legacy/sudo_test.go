package legacy

import (
	"reflect"
	"slices"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func TestParseSudo(t *testing.T) {
	sudo := func(text string, a protocol.SudoAction, name, arg string) protocol.Sudo {
		return protocol.Sudo{Text: text, Action: a, Name: name, Arg: arg}
	}
	for _, tc := range []struct {
		line string
		want protocol.Command
	}{
		{"sudo gag alice", sudo("sudo gag alice", protocol.SudoGag, "alice", "")},
		{"sudo ungag alice", sudo("sudo ungag alice", protocol.SudoUngag, "alice", "")},
		{"sudo kick Guest12", sudo("sudo kick Guest12", protocol.SudoKick, "Guest12", "")},
		{"sudo ban alice spamming chat", sudo("sudo ban alice spamming chat", protocol.SudoBan, "alice", "spamming chat")},
		{"sudo unban alice", sudo("sudo unban alice", protocol.SudoUnban, "alice", "")},
		{"sudo list gag", sudo("sudo list gag", protocol.SudoList, "", "gag")},
		{"sudo list online", sudo("sudo list online", protocol.SudoList, "", "online")},
		{"sudo broadcast Server going down", sudo("sudo broadcast Server going down", protocol.SudoBroadcast, "", "Server going down")},
		// The bad word filter was dropped, so its reload command is unknown.
		{"sudo reload wordconfig", sudo("sudo reload wordconfig", protocol.SudoUnknown, "", "")},
		{"sudo mod alice", sudo("sudo mod alice", protocol.SudoMod, "alice", "")},
		{"sudo unmod alice", sudo("sudo unmod alice", protocol.SudoUnmod, "alice", "")},
		{"sudo admin alice", sudo("sudo admin alice", protocol.SudoAdmin, "alice", "")},
		{"sudo unadmin alice", sudo("sudo unadmin alice", protocol.SudoUnadmin, "alice", "")},
		{"sudo bot alice", sudo("sudo bot alice", protocol.SudoBot, "alice", "")},
		{"sudo unbot alice", sudo("sudo unbot alice", protocol.SudoUnbot, "alice", "")},
		{"sudo revoke alice", sudo("sudo revoke alice", protocol.SudoRevoke, "alice", "")},
		// The password never appears in Text.
		{"sudo set password alice s3cretpw", sudo("sudo set password alice ******", protocol.SudoSetPassword, "alice", "s3cretpw")},
		{"sudo set password alice short", sudo("sudo set password alice ******", protocol.SudoUnknown, "", "")},
		{"sudo set colour alice s3cretpw", sudo("sudo set colour alice ******", protocol.SudoUnknown, "", "")},
		// Typos and incomplete commands are Unknown, answered "Command not found".
		{"sudo gagg alice", sudo("sudo gagg alice", protocol.SudoUnknown, "", "")},
		{"sudo ban alice", sudo("sudo ban alice", protocol.SudoUnknown, "", "")},
		{"sudo kick al", sudo("sudo kick al", protocol.SudoUnknown, "", "")},
		{"sudo whatever", sudo("sudo whatever", protocol.SudoUnknown, "", "")},
		{"sudo ", nil},
		{"sudo", nil},
		{"Shout sudo gag alice", protocol.PostShout{Text: "sudo gag alice"}},
	} {
		if got := Parse(tc.line, true); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q) = %#v, want %#v", tc.line, got, tc.want)
		}
	}
	if Parse("sudo list online", false) != nil {
		t.Error("sudo requires login")
	}
}

func TestFormatSudoReply(t *testing.T) {
	got, err := Format(protocol.SudoReply{Text: "> sudo list mod"}, 0)
	if err != nil || !slices.Equal(got, []string{"sudoReply > sudo list mod"}) {
		t.Errorf("%q %v", got, err)
	}
}
