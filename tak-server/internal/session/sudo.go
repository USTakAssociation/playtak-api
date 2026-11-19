package session

import (
	"context"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/moderation"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// sudo runs a moderator or admin command. Anyone else gets NOK; otherwise
// the command is echoed, then its result.
func (s *Session) sudo(ctx context.Context, p *online.Player, command protocol.Sudo) {
	if !moderation.Allowed(p) {
		s.Send(protocol.NOK{})
		return
	}
	s.Send(protocol.SudoReply{Text: "> " + command.Text})
	r := s.manager.moderation.Run(ctx, p, command)
	for _, line := range r.Replies {
		s.Send(protocol.SudoReply{Text: line})
	}
	if r.Broadcast != "" {
		for _, o := range s.manager.readySessions() {
			o.Send(protocol.RawLine{Text: r.Broadcast})
		}
	}
}
