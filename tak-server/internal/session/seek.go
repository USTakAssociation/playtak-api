package session

import (
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
)

// Players can't seek while in a game.
func (s *Session) postSeek(p *online.Player, command protocol.PostSeek) {
	var err error
	idle := s.manager.games.IfIdle(p.Name, func() {
		_, err = s.manager.seeks.Post(s, seek.Request{Creator: p.Name, Bot: p.IsBot(), Opponent: command.Opponent, Settings: command.Settings})
	})
	if !idle || err != nil {
		s.Send(protocol.NOK{})
	}
}

func (s *Session) rematch(p *online.Player, command protocol.Rematch) {
	var result seek.RematchResult
	var err error
	idle := s.manager.games.IfIdle(p.Name, func() {
		result, err = s.manager.seeks.Rematch(s, command.GameID, seek.Request{Creator: p.Name, Bot: p.IsBot(), Opponent: command.Opponent, Settings: command.Settings})
	})
	switch {
	case !idle || err != nil:
		s.Send(protocol.NOK{})
	case result.AcceptID != 0:
		s.Send(protocol.AcceptRematch{SeekID: result.AcceptID})
	default:
		s.Send(protocol.RematchSeekCreated{SeekID: result.Created.ID})
	}
}
