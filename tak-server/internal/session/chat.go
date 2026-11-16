package session

import (
	"github.com/USTakAssociation/playtak-api/tak-server/internal/chat"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func (s *Session) chatSender(p *online.Player) chat.Sender {
	return chat.Sender{Name: p.Name, Gagged: p.IsGagged(), Session: s}
}

func (s *Session) shout(player *online.Player, command protocol.PostShout) {
	var everyone []chat.Member
	for _, other := range s.manager.readySessions() {
		everyone = append(everyone, other)
	}
	s.manager.chat.Shout(s.chatSender(player), everyone, command.Text)
}

func (s *Session) tell(player *online.Player, command protocol.SendTell) {
	var err error
	if target := s.manager.players.FindOnline(command.To); target != nil {
		err = s.manager.chat.Tell(s.chatSender(player), target.Name, target, command.Text)
	} else {
		err = s.manager.chat.Tell(s.chatSender(player), command.To, nil, command.Text)
	}
	if err != nil {
		s.Send(protocol.CommandFailed{Reason: err.Error()})
	}
}
