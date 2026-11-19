// Package chat delivers shouts, room shouts and private tells, keeping
// gagged players' messages to themselves.
package chat

import (
	"errors"
	"log/slog"
	"sort"
	"sync"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// MaxMessage is the longest message in characters; longer ones are cut, as
// the Java server's regular expressions did.
const MaxMessage = 256

// mutedNotice replaces a gagged player's message in the copy only they see,
// so they aren't told their messages go nowhere.
const mutedNotice = "<Server: You have been muted for inappropriate chat behavior.>"

var (
	// ErrNotOnline means the player to tell isn't logged in.
	ErrNotOnline = errors.New("Player is not online")
	// ErrNotInRoom means the sender hasn't joined the room.
	ErrNotInRoom = errors.New("You are not in that room")
)

// Member is a session that can receive chat.
type Member interface {
	Send(event protocol.Event)
}

// Sender is who a message comes from.
type Sender struct {
	Name    string
	Gagged  bool
	Session Member // where the sender's own copy goes
}

// Service delivers chat. Every message passes through it, which is where
// chat history would be recorded.
type Service struct {
	log *slog.Logger

	mu    sync.Mutex
	rooms map[string]map[Member]struct{}
}

func NewService(log *slog.Logger) *Service {
	return &Service{log: log, rooms: make(map[string]map[Member]struct{})}
}

// Shout sends text to everyone, or only back to a gagged sender.
func (s *Service) Shout(from Sender, everyone []Member, text string) {
	text = truncate(text)
	s.log.Info("shout", "player", from.Name, "gagged", from.Gagged, "text", text)
	if from.Gagged {
		from.Session.Send(protocol.Shout{From: from.Name, Text: mutedNotice})
		return
	}
	event := protocol.Shout{From: from.Name, Text: text}
	for _, m := range everyone {
		m.Send(event)
	}
}

// Join adds m to room, creating it if needed.
func (s *Service) Join(room string, member Member) {
	s.mu.Lock()
	members := s.rooms[room]
	if members == nil {
		members = make(map[Member]struct{})
		s.rooms[room] = members
	}
	members[member] = struct{}{}
	s.mu.Unlock()
	member.Send(protocol.RoomJoined{Room: room})
}

// Leave removes m from room; an empty room is deleted.
func (s *Service) Leave(room string, m Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaveLocked(room, m)
}

// LeaveAll removes m from every room, e.g. when it disconnects.
func (s *Service) LeaveAll(m Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for room := range s.rooms {
		s.leaveLocked(room, m)
	}
}

// JoinedRooms returns the rooms m is in, sorted.
func (s *Service) JoinedRooms(m Member) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var rooms []string
	for room, members := range s.rooms {
		if _, ok := members[m]; ok {
			rooms = append(rooms, room)
		}
	}
	sort.Strings(rooms)
	return rooms
}

func (s *Service) leaveLocked(room string, m Member) {
	delete(s.rooms[room], m)
	if len(s.rooms[room]) == 0 {
		delete(s.rooms, room)
	}
}

// ShoutRoom sends text to room's members. The sender must have joined it.
func (s *Service) ShoutRoom(from Sender, room, text string) error {
	text = truncate(text)
	s.mu.Lock()
	_, member := s.rooms[room][from.Session]
	var members []Member
	for m := range s.rooms[room] {
		members = append(members, m)
	}
	s.mu.Unlock()
	if !member {
		return ErrNotInRoom
	}
	s.log.Info("room shout", "player", from.Name, "room", room, "gagged", from.Gagged, "text", text)
	if from.Gagged {
		from.Session.Send(protocol.RoomShout{Room: room, From: from.Name, Text: mutedNotice})
		return nil
	}
	event := protocol.RoomShout{Room: room, From: from.Name, Text: text}
	for _, m := range members {
		m.Send(event)
	}
	return nil
}

// Recipient is a player who can be told something: all of their sessions
// receive it.
type Recipient interface {
	Send(event protocol.Event)
}

// Tell sends text privately to the player named to, who must be online.
// The sender's session gets a Told copy. Message text isn't logged.
func (s *Service) Tell(from Sender, to string, target Recipient, text string) error {
	if target == nil {
		return ErrNotOnline
	}
	text = truncate(text)
	s.log.Info("tell", "player", from.Name, "to", to, "gagged", from.Gagged)
	if from.Gagged {
		from.Session.Send(protocol.Told{To: to, Text: mutedNotice})
		return nil
	}
	target.Send(protocol.TellReceived{From: from.Name, Text: text})
	from.Session.Send(protocol.Told{To: to, Text: text})
	return nil
}

// truncate cuts text to MaxMessage characters.
func truncate(text string) string {
	n := 0
	for i := range text {
		if n == MaxMessage {
			return text[:i]
		}
		n++
	}
	return text
}
