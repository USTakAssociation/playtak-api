package legacy

import (
	"regexp"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// The Java server's patterns. Messages longer than 256 characters and room
// names longer than 64 match only up to that length, so they are cut.
var (
	reShout     = regexp.MustCompile(`^Shout ([^\n\r]{1,256})`)
	reShoutRoom = regexp.MustCompile(`^ShoutRoom ([^\n\r\s]{1,64}) ([^\n\r]{1,256})`)
	reJoinRoom  = regexp.MustCompile(`^JoinRoom ([^\n\r\s]{1,64})`)
	reLeaveRoom = regexp.MustCompile(`^LeaveRoom ([^\n\r\s]{1,64})`)
	reTell      = regexp.MustCompile(`^Tell ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\n\r]{1,256})`)
)

// parseChatCommand parses chat commands, or returns nil.
func parseChatCommand(line string) protocol.Command {
	if m := reShoutRoom.FindStringSubmatch(line); m != nil {
		return protocol.PostRoomShout{Room: m[1], Text: m[2]}
	}
	if m := reShout.FindStringSubmatch(line); m != nil {
		return protocol.PostShout{Text: m[1]}
	}
	if m := reJoinRoom.FindStringSubmatch(line); m != nil {
		return protocol.JoinRoom{Room: m[1]}
	}
	if m := reLeaveRoom.FindStringSubmatch(line); m != nil {
		return protocol.LeaveRoom{Room: m[1]}
	}
	if m := reTell.FindStringSubmatch(line); m != nil {
		return protocol.SendTell{To: m[1], Text: m[2]}
	}
	return nil
}
