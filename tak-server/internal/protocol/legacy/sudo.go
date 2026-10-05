package legacy

import (
	"regexp"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

const sudoName = `([a-zA-Z][a-zA-Z0-9_]{3,15})`

var (
	reSudo       = regexp.MustCompile(`^sudo ([^\n\r]{1,256})`)
	reSudoBan    = regexp.MustCompile(`^sudo ban ` + sudoName + ` ([^\n\r]{1,256})`)
	reSudoList   = regexp.MustCompile(`^sudo list ([a-zA-Z]{3,15})`)
	reSudoReload = regexp.MustCompile(`^sudo reload wordconfig`)
	reSudoSet    = regexp.MustCompile(`^sudo set ([a-zA-Z]{3,15}) ` + sudoName + ` ([^\n\r\s]{6,100})`)
	reSudoSetAny = regexp.MustCompile(`^(sudo set \S+ \S+ )\S+`)
	reSudoBcast  = regexp.MustCompile(`^sudo broadcast ([^\n\r]{1,256})`)
	sudoNameOnly = map[string]protocol.SudoAction{
		"gag": protocol.SudoGag, "ungag": protocol.SudoUngag, "kick": protocol.SudoKick,
		"unban": protocol.SudoUnban, "mod": protocol.SudoMod, "unmod": protocol.SudoUnmod,
		"admin": protocol.SudoAdmin, "unadmin": protocol.SudoUnadmin, "bot": protocol.SudoBot,
		"unbot": protocol.SudoUnbot, "revoke": protocol.SudoRevoke,
	}
	reSudoNameOnly = regexp.MustCompile(`^sudo ([a-z]+) ` + sudoName)
)

// parseSudo parses a sudo command, or returns nil. A line starting "sudo "
// that isn't a known command is a Sudo with SudoUnknown, so it can be
// answered "Command not found".
func parseSudo(line string) protocol.Command {
	if !reSudo.MatchString(line) {
		return nil
	}
	// Hide a password wherever this line is echoed or logged, even if it
	// isn't a valid set command.
	text := reSudoSetAny.ReplaceAllString(line, "${1}******")
	cmd := protocol.Sudo{Text: text}
	if m := reSudoBan.FindStringSubmatch(line); m != nil {
		cmd.Action, cmd.Name, cmd.Arg = protocol.SudoBan, m[1], m[2]
	} else if m := reSudoList.FindStringSubmatch(line); m != nil {
		cmd.Action, cmd.Arg = protocol.SudoList, m[1]
	} else if reSudoReload.MatchString(line) {
		cmd.Action = protocol.SudoReloadWords
	} else if m := reSudoSet.FindStringSubmatch(line); m != nil {
		if m[1] == "password" {
			cmd.Action, cmd.Name, cmd.Arg = protocol.SudoSetPassword, m[2], m[3]
		}
	} else if m := reSudoBcast.FindStringSubmatch(line); m != nil {
		cmd.Action, cmd.Arg = protocol.SudoBroadcast, m[1]
	} else if m := reSudoNameOnly.FindStringSubmatch(line); m != nil {
		if a, ok := sudoNameOnly[m[1]]; ok {
			cmd.Action, cmd.Name = a, m[2]
		}
	}
	return cmd
}
