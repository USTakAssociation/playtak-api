package chat

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

type member struct {
	mu     sync.Mutex
	events []protocol.Event
}

func (m *member) Send(ev protocol.Event) { m.mu.Lock(); m.events = append(m.events, ev); m.mu.Unlock() }

func (m *member) take() []protocol.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev := m.events
	m.events = nil
	return ev
}

func newService() (*Service, *bytes.Buffer) {
	var log bytes.Buffer
	return NewService(slog.New(slog.NewTextHandler(&log, nil))), &log
}

func (s *Service) roomCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rooms)
}

func TestShout(t *testing.T) {
	s, log := newService()
	a, b := &member{}, &member{}
	s.Shout(Sender{Name: "alice", Session: a}, []Member{a, b}, "hello")
	want := []protocol.Event{protocol.Shout{From: "alice", Text: "hello"}}
	for _, m := range []*member{a, b} {
		if got := m.take(); !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	// Logged unmasked, for moderators.
	if !strings.Contains(log.String(), `msg=shout player=alice gagged=false text=hello`) {
		t.Errorf("log: %s", log.String())
	}
}

func TestGaggedShoutOnlyEchoes(t *testing.T) {
	s, log := newService()
	a, b := &member{}, &member{}
	s.Shout(Sender{Name: "alice", Gagged: true, Session: a}, []Member{a, b}, "hello")
	if got := a.take(); !slices.Equal(got, []protocol.Event{protocol.Shout{From: "alice", Text: mutedNotice}}) {
		t.Errorf("sender got %v", got)
	}
	if got := b.take(); len(got) != 0 {
		t.Errorf("others got %v", got)
	}
	if !strings.Contains(log.String(), "gagged=true") {
		t.Errorf("gagged shouts are still logged: %s", log.String())
	}
}

func TestRooms(t *testing.T) {
	s, _ := newService()
	a, b, c := &member{}, &member{}, &member{}
	s.Join("lobby", a)
	s.Join("lobby", b)
	s.Join("lobby", a) // joining twice is harmless
	if got := a.take(); !slices.Equal(got, []protocol.Event{protocol.RoomJoined{Room: "lobby"}, protocol.RoomJoined{Room: "lobby"}}) {
		t.Errorf("joins: %v", got)
	}
	b.take()

	if err := s.ShoutRoom(Sender{Name: "alice", Session: a}, "lobby", "hi"); err != nil {
		t.Fatal(err)
	}
	want := []protocol.Event{protocol.RoomShout{Room: "lobby", From: "alice", Text: "hi"}}
	for _, m := range []*member{a, b} {
		if got := m.take(); !slices.Equal(got, want) {
			t.Errorf("member got %v", got)
		}
	}

	// Non-members can't shout into a room, and don't hear it.
	if err := s.ShoutRoom(Sender{Name: "carol", Session: c}, "lobby", "hi"); !errors.Is(err, ErrNotInRoom) {
		t.Errorf("non-member: %v", err)
	}
	if err := s.ShoutRoom(Sender{Name: "carol", Session: c}, "nowhere", "hi"); !errors.Is(err, ErrNotInRoom) {
		t.Errorf("missing room: %v", err)
	}
	if len(a.take())+len(b.take())+len(c.take()) != 0 {
		t.Error("refused shouts must not be delivered")
	}

	s.Leave("lobby", b)
	s.ShoutRoom(Sender{Name: "alice", Session: a}, "lobby", "bye")
	if got := b.take(); len(got) != 0 {
		t.Errorf("left member got %v", got)
	}
	if err := s.ShoutRoom(Sender{Name: "bob", Session: b}, "lobby", "hi"); !errors.Is(err, ErrNotInRoom) {
		t.Errorf("after leaving: %v", err)
	}

	// The room goes away with its last member; leaving again is harmless.
	s.Leave("lobby", a)
	s.Leave("lobby", a)
	s.Leave("never-joined", a)
	if n := s.roomCount(); n != 0 {
		t.Errorf("%d rooms left", n)
	}
}

func TestGaggedRoomShout(t *testing.T) {
	s, _ := newService()
	a, b := &member{}, &member{}
	s.Join("lobby", a)
	s.Join("lobby", b)
	a.take()
	b.take()
	if err := s.ShoutRoom(Sender{Name: "alice", Gagged: true, Session: a}, "lobby", "hi"); err != nil {
		t.Fatal(err)
	}
	if got := a.take(); !slices.Equal(got, []protocol.Event{protocol.RoomShout{Room: "lobby", From: "alice", Text: mutedNotice}}) {
		t.Errorf("sender got %v", got)
	}
	if got := b.take(); len(got) != 0 {
		t.Errorf("member got %v", got)
	}
}

func TestLeaveAll(t *testing.T) {
	s, _ := newService()
	a, b := &member{}, &member{}
	s.Join("one", a)
	s.Join("two", a)
	s.Join("two", b)
	if got := s.JoinedRooms(a); !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("a's rooms %v", got)
	}
	s.LeaveAll(a)
	if got := s.JoinedRooms(a); len(got) != 0 {
		t.Errorf("a's rooms after LeaveAll %v", got)
	}
	if n := s.roomCount(); n != 1 {
		t.Errorf("%d rooms, want only b's", n)
	}
	b.take()
	s.ShoutRoom(Sender{Name: "bob", Session: b}, "two", "hi")
	if got := a.take(); len(got) != 2 { // only the two join confirmations
		t.Errorf("a got %v", got)
	}
}

func TestTell(t *testing.T) {
	s, log := newService()
	sender, target := &member{}, &member{}
	if err := s.Tell(Sender{Name: "alice", Session: sender}, "Bobby", target, "secret"); err != nil {
		t.Fatal(err)
	}
	if got := target.take(); !slices.Equal(got, []protocol.Event{protocol.TellReceived{From: "alice", Text: "secret"}}) {
		t.Errorf("target got %v", got)
	}
	if got := sender.take(); !slices.Equal(got, []protocol.Event{protocol.Told{To: "Bobby", Text: "secret"}}) {
		t.Errorf("sender got %v", got)
	}
	if strings.Contains(strings.ToLower(log.String()), "secret") {
		t.Errorf("tell text must not be logged: %s", log.String())
	}
	if !strings.Contains(log.String(), "msg=tell player=alice to=Bobby") {
		t.Errorf("log: %s", log.String())
	}
}

func TestTellOffline(t *testing.T) {
	s, _ := newService()
	sender := &member{}
	if err := s.Tell(Sender{Name: "alice", Session: sender}, "bobby", nil, "hi"); !errors.Is(err, ErrNotOnline) {
		t.Errorf("err %v", err)
	}
	if got := sender.take(); len(got) != 0 {
		t.Errorf("sender got %v", got)
	}
}

func TestGaggedTell(t *testing.T) {
	s, _ := newService()
	sender, target := &member{}, &member{}
	s.Tell(Sender{Name: "alice", Gagged: true, Session: sender}, "bobby", target, "hi")
	if got := sender.take(); !slices.Equal(got, []protocol.Event{protocol.Told{To: "bobby", Text: mutedNotice}}) {
		t.Errorf("sender got %v", got)
	}
	if got := target.take(); len(got) != 0 {
		t.Errorf("target got %v", got)
	}
}

func TestMessagesAreCut(t *testing.T) {
	s, _ := newService()
	a := &member{}
	long := strings.Repeat("😀", 300)
	s.Shout(Sender{Name: "alice", Session: a}, []Member{a}, long)
	got := a.take()[0].(protocol.Shout).Text
	if n := len([]rune(got)); n != MaxMessage {
		t.Errorf("%d characters, want %d", n, MaxMessage)
	}
	for in, want := range map[string]string{
		"":                             "",
		"short":                        "short",
		strings.Repeat("a", 256):       strings.Repeat("a", 256),
		strings.Repeat("a", 257):       strings.Repeat("a", 256),
		strings.Repeat("é", 257) + "x": strings.Repeat("é", 256),
	} {
		if got := truncate(in); got != want {
			t.Errorf("truncate(%d runes) = %d runes", len([]rune(in)), len([]rune(got)))
		}
	}
}

func TestConcurrentRooms(t *testing.T) {
	s, _ := newService()
	var wg sync.WaitGroup
	for range 20 {
		m := &member{}
		wg.Go(func() {
			for range 50 {
				s.Join("busy", m)
				s.ShoutRoom(Sender{Name: "x", Session: m}, "busy", "hi")
				s.LeaveAll(m)
			}
		})
	}
	wg.Wait()
	if n := s.roomCount(); n != 0 {
		t.Errorf("%d rooms left", n)
	}
}

// Shouts, room shouts and tells share one limit per player, so switching
// between them doesn't get around it, and one player's flood doesn't limit
// anyone else.
func TestMessagesRateLimitedPerPlayer(t *testing.T) {
	s, _ := newService()
	a, b := &member{}, &member{}
	alice := Sender{Name: "alice", Session: a}
	s.Join("room", a)
	a.take()

	for i := range messagesPerPlayer {
		var err error
		switch i % 3 {
		case 0:
			err = s.Shout(alice, []Member{a, b}, "hi")
		case 1:
			err = s.ShoutRoom(alice, "room", "hi")
		case 2:
			err = s.Tell(alice, "bob", b, "hi")
		}
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
	}
	b.take()
	if err := s.Shout(alice, []Member{a, b}, "spam"); !errors.Is(err, ErrTooFast) {
		t.Errorf("shout over the limit: got %v, want ErrTooFast", err)
	}
	if err := s.ShoutRoom(alice, "room", "spam"); !errors.Is(err, ErrTooFast) {
		t.Errorf("room shout over the limit: got %v, want ErrTooFast", err)
	}
	if err := s.Tell(alice, "bob", b, "spam"); !errors.Is(err, ErrTooFast) {
		t.Errorf("tell over the limit: got %v, want ErrTooFast", err)
	}
	if got := b.take(); len(got) != 0 {
		t.Errorf("limited messages were delivered: %v", got)
	}
	if err := s.Shout(Sender{Name: "bob", Session: b}, []Member{a, b}, "hi"); err != nil {
		t.Errorf("bob is limited by alice's flood: %v", err)
	}
}
