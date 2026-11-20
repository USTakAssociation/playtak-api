// Package chat delivers shouts, room shouts and private tells, keeping
// gagged players' messages to themselves.
package chat

import (
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/ratelimit"
)

// MaxMessage is the longest message in characters; longer ones are cut, as
// the Java server's regular expressions did.
const MaxMessage = 256

// mutedNotice replaces a gagged player's message in the copy only they see,
// so they aren't told their messages go nowhere.
const mutedNotice = "<Server: You have been muted for inappropriate chat behavior.>"

// Each player may send messagesPerPlayer messages (shouts, room shouts and
// tells together) per messagesPeriod, all at once if they like. One shout
// reaches every session, so a flood could otherwise fill slow clients' send
// queues and get them disconnected.
const (
	messagesPerPlayer = 10
	messagesPeriod    = 10 * time.Second
	maxTrackedPlayers = 100_000
	// maxRoomsPerMember bounds how many rooms one session can be in, so a
	// client can't grow the room table without limit. Clients join a room
	// per observed game.
	maxRoomsPerMember = 50
)

var (
	// ErrNotOnline means the player to tell isn't logged in.
	ErrNotOnline = errors.New("Player is not online")
	// ErrNotInRoom means the sender hasn't joined the room.
	ErrNotInRoom = errors.New("You are not in that room")
	// ErrTooFast means the sender has used up their messages for now.
	ErrTooFast = errors.New("You are sending messages too fast")
	// ErrTooManyRooms means the session is already in maxRoomsPerMember rooms.
	ErrTooManyRooms = errors.New("You are in too many rooms")
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
	log   *slog.Logger
	limit *ratelimit.Keyed // messages per player name

	mu     sync.Mutex
	rooms  map[string]map[Member]struct{}
	joined map[Member]int // how many rooms each member is in
}

func NewService(log *slog.Logger) *Service {
	return &Service{
		log:    log,
		limit:  ratelimit.New(messagesPerPlayer, messagesPeriod, maxTrackedPlayers),
		rooms:  make(map[string]map[Member]struct{}),
		joined: make(map[Member]int),
	}
}

// PruneLimits forgets players whose message limits have reset.
func (s *Service) PruneLimits() { s.limit.Prune(s.log, "chat messages per player") }

// allow takes one message from the sender's limit, or returns ErrTooFast.
func (s *Service) allow(from Sender) error {
	if ok, _ := s.limit.Allow(from.Name); !ok {
		return ErrTooFast
	}
	return nil
}

// Shout sends text to everyone, or only back to a gagged sender.
func (s *Service) Shout(from Sender, everyone []Member, text string) error {
	if err := s.allow(from); err != nil {
		return err
	}
	text = truncate(text)
	s.log.Info("shout", "player", from.Name, "gagged", from.Gagged, "text", text)
	if from.Gagged {
		from.Session.Send(protocol.Shout{From: from.Name, Text: mutedNotice})
		return nil
	}
	event := protocol.Shout{From: from.Name, Text: text}
	for _, m := range everyone {
		m.Send(event)
	}
	return nil
}

// Join adds m to room, creating it if needed. A member already in
// maxRoomsPerMember rooms gets ErrTooManyRooms.
func (s *Service) Join(room string, member Member) error {
	s.mu.Lock()
	members := s.rooms[room]
	if _, in := members[member]; !in {
		if s.joined[member] >= maxRoomsPerMember {
			s.mu.Unlock()
			return ErrTooManyRooms
		}
		if members == nil {
			members = make(map[Member]struct{})
			s.rooms[room] = members
		}
		members[member] = struct{}{}
		s.joined[member]++
	}
	s.mu.Unlock()
	member.Send(protocol.RoomJoined{Room: room})
	return nil
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
	members, ok := s.rooms[room]
	if !ok {
		return
	}
	if _, in := members[m]; in {
		delete(members, m)
		if s.joined[m]--; s.joined[m] <= 0 {
			delete(s.joined, m)
		}
	}
	if len(members) == 0 {
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
	if err := s.allow(from); err != nil {
		return err
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
	if err := s.allow(from); err != nil {
		return err
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
