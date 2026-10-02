// Package legacy implements the original line-based text protocol spoken by
// existing clients. Its parsing and output must stay byte-for-byte compatible
// with the Java server.
package legacy

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Patterns match a prefix of the line (anything after the match is ignored)
// and are tried in order, so both the patterns and their order are part of
// the protocol. `[^\s\x0B]` is Java's \S, which Go's \S doesn't quite match.
var (
	rePing           = regexp.MustCompile(`^PING$`)
	reClient         = regexp.MustCompile(`^Client ([A-Za-z.0-9-]{1,60})`)
	reProtocol       = regexp.MustCompile(`^Protocol ([0-9]{1,8})`)
	reLoginGuest     = regexp.MustCompile(`^Login Guest ?(([a-z]{20})?)`)
	reLogin          = regexp.MustCompile(`^Login ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\s\x0B]{6,50})`)
	reRegister       = regexp.MustCompile(`^Register ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([A-Za-z.0-9_+!#$%&'*^?=-]{1,30}@[A-Za-z.0-9-]{3,30})`)
	reRegisterAny    = regexp.MustCompile(`^Register [^\n\r]{1,256}`)
	reSendResetToken = regexp.MustCompile(`^SendResetToken ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([A-Za-z.0-9_+!#$%&'*^?=-]{1,30}@[A-Za-z.0-9-]{3,30})`)
	reResetPassword  = regexp.MustCompile(`^ResetPassword ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\s\x0B]{6,50}) ([^\s\x0B]{6,50})`)
	reChangePassword = regexp.MustCompile(`^ChangePassword ([^\s\x0B]{6,50}) ([^\s\x0B]{6,50})`)
)

// Parse converts one line from a client into a command. loggedIn selects
// which commands are available. It returns nil if the line isn't a command
// the server understands, which clients are told with NOK.
func Parse(line string, loggedIn bool) protocol.Command {
	line = strings.TrimRight(line, "\r\n")
	switch {
	case line == "quit":
		return protocol.Quit{}
	case rePing.MatchString(line):
		return protocol.Ping{}
	case loggedIn:
		return parseLoggedIn(line)
	default:
		return parseLoggedOut(line)
	}
}

func parseLoggedOut(line string) protocol.Command {
	if m := reClient.FindStringSubmatch(line); m != nil {
		return protocol.ClientName{Name: m[1]}
	}
	if m := reProtocol.FindStringSubmatch(line); m != nil {
		v, _ := strconv.Atoi(m[1]) // at most 8 digits, always fits
		return protocol.SetProtocol{Version: v}
	}
	if m := reLoginGuest.FindStringSubmatch(line); m != nil {
		return protocol.LoginGuest{Token: m[1]}
	}
	if m := reLogin.FindStringSubmatch(line); m != nil {
		return protocol.Login{Name: m[1], Password: m[2]}
	}
	if m := reRegister.FindStringSubmatch(line); m != nil {
		return protocol.Register{Name: m[1], Email: m[2]}
	}
	if reRegisterAny.MatchString(line) {
		return protocol.RegisterMalformed{}
	}
	if m := reSendResetToken.FindStringSubmatch(line); m != nil {
		return protocol.SendResetToken{Name: m[1], Email: m[2]}
	}
	if m := reResetPassword.FindStringSubmatch(line); m != nil {
		return protocol.ResetPassword{Name: m[1], Token: m[2], Password: m[3]}
	}
	return nil
}

func parseLoggedIn(line string) protocol.Command {
	if cmd := parseSeekCommand(line); cmd != nil {
		return cmd
	}
	if cmd := parseGameCommand(line); cmd != nil {
		return cmd
	}
	if m := reChangePassword.FindStringSubmatch(line); m != nil {
		return protocol.ChangePassword{Old: m[1], New: m[2]}
	}
	return nil
}
