package seek

import (
	"errors"
	"reflect"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

type listener struct {
	version int
	events  []protocol.Event
}

func (l *listener) Send(ev protocol.Event) { l.events = append(l.events, ev) }
func (l *listener) ProtocolVersion() int   { return l.version }

func (l *listener) take() []protocol.Event {
	ev := l.events
	l.events = nil
	return ev
}

var std5 = protocol.GameSettings{Size: 5, Time: 600, Increment: 30, Pieces: 21, Capstones: 1}

func TestPostReplacesAndBroadcasts(t *testing.T) {
	m := NewManager()
	owner, watcher := &listener{}, &listener{}
	m.Subscribe(owner)
	m.Subscribe(watcher)

	first, err := m.Post(owner, Request{Creator: "alice", Settings: std5})
	if err != nil {
		t.Fatal(err)
	}
	if got := watcher.take(); !reflect.DeepEqual(got, []protocol.Event{protocol.SeekAdded{Seek: first.Seek}}) {
		t.Errorf("after first post: %v", got)
	}

	second, _ := m.Post(owner, Request{Creator: "alice", Settings: std5})
	want := []protocol.Event{protocol.SeekRemoved{Seek: first.Seek}, protocol.SeekAdded{Seek: second.Seek}}
	if got := watcher.take(); !reflect.DeepEqual(got, want) {
		t.Errorf("after replacing: %v", got)
	}
	if second.ID != first.ID+1 {
		t.Errorf("ids %d, %d", first.ID, second.ID)
	}

	m.Cancel(owner)
	if got := watcher.take(); !reflect.DeepEqual(got, []protocol.Event{protocol.SeekRemoved{Seek: second.Seek}}) {
		t.Errorf("after cancel: %v", got)
	}
	m.Cancel(owner) // no seek: nothing happens
	if got := watcher.take(); len(got) != 0 {
		t.Errorf("cancel without a seek broadcast %v", got)
	}
}

func TestOneSeekPerOwner(t *testing.T) {
	m := NewManager()
	tab1, tab2 := &listener{}, &listener{}
	m.Post(tab1, Request{Creator: "alice", Settings: std5})
	m.Post(tab2, Request{Creator: "alice", Settings: std5})
	if n := len(m.List()); n != 2 {
		t.Errorf("two sessions should hold two seeks, got %d", n)
	}
	m.Cancel(tab1)
	if l := m.List(); len(l) != 1 || l[0].Owner != tab2 {
		t.Errorf("cancel should only remove the owner's seek: %v", l)
	}
}

func TestSubscribeSendsCurrentList(t *testing.T) {
	m := NewManager()
	a, _ := m.Post(&listener{}, Request{Creator: "alice", Settings: std5})
	b, _ := m.Post(&listener{}, Request{Creator: "bob", Settings: std5})

	l := &listener{}
	m.Subscribe(l)
	want := []protocol.Event{protocol.SeekAdded{Seek: a.Seek}, protocol.SeekAdded{Seek: b.Seek}}
	if got := l.take(); !reflect.DeepEqual(got, want) {
		t.Errorf("Subscribe sent %v", got)
	}
	m.Unsubscribe(l)
	m.Post(&listener{}, Request{Creator: "carol", Settings: std5})
	if got := l.take(); len(got) != 0 {
		t.Errorf("unsubscribed listener got %v", got)
	}
}

func TestProtocolGating(t *testing.T) {
	m := NewManager()
	old, modern := &listener{version: 3}, &listener{version: 4}
	m.Subscribe(old)
	m.Subscribe(modern)

	dbs := std5
	dbs.Opening = protocol.OpeningDoubleBlackStack
	if _, err := m.Post(old, Request{Creator: "x", Settings: dbs}); !errors.Is(err, ErrIncompatible) {
		t.Errorf("old client posted an opening it can't display: %v", err)
	}

	if _, err := m.Post(modern, Request{Creator: "y", Settings: dbs}); err != nil {
		t.Fatal(err)
	}
	if got := old.take(); len(got) != 0 {
		t.Errorf("old client was sent %v", got)
	}
	if got := modern.take(); len(got) != 1 {
		t.Errorf("modern client got %v", got)
	}
	m.SendList(old)
	if got := old.take(); len(got) != 0 {
		t.Errorf("List showed an incompatible seek: %v", got)
	}
	m.Cancel(modern)
	if got := old.take(); len(got) != 0 {
		t.Errorf("old client got a remove for a seek it never saw: %v", got)
	}

	scaled := std5
	scaled.IncrementScales = true // clock-only, so shown to everyone
	m.Post(modern, Request{Creator: "y", Settings: scaled})
	if got := old.take(); len(got) != 1 {
		t.Errorf("scaled increment should be visible to old clients, got %v", got)
	}
}

func TestNormalize(t *testing.T) {
	m := NewManager()
	s, _ := m.Post(&listener{}, Request{Creator: "x", Settings: protocol.GameSettings{
		Size: 9, Komi: 20, Pieces: 200, Capstones: 9,
	}})
	g := s.GameSettings
	if g.Size != 5 || g.Komi != 8 || g.Pieces != 80 || g.Capstones != 5 {
		t.Errorf("not clamped: %+v", g)
	}
	s, _ = m.Post(&listener{}, Request{Creator: "x", Settings: protocol.GameSettings{Size: 3, Pieces: 2}})
	if s.Size != 3 || s.Pieces != 10 {
		t.Errorf("minimums: %+v", s.GameSettings)
	}
}

func TestRematch(t *testing.T) {
	m := NewManager()
	alice, bob := &listener{}, &listener{}
	m.Subscribe(alice)
	m.Subscribe(bob)
	m.Post(alice, Request{Creator: "alice", Settings: std5}) // replaced by the rematch

	res, err := m.Rematch(alice, 42, Request{Creator: "alice", Opponent: "bob", Settings: std5})
	if err != nil || res.AcceptID != 0 || res.Created.RematchOf != 42 {
		t.Fatalf("first rematch: %+v, %v", res, err)
	}
	if n := len(m.List()); n != 1 {
		t.Errorf("alice's earlier seek should be replaced, have %d seeks", n)
	}

	// Bob asks too: he's told to accept alice's seek (opponent match ignores case).
	res, err = m.Rematch(bob, 42, Request{Creator: "BOB", Opponent: "alice", Settings: std5})
	if err != nil || res.AcceptID == 0 || res.Created.ID != 0 {
		t.Fatalf("second rematch: %+v, %v", res, err)
	}
	if s, ok := m.Get(res.AcceptID); !ok || s.Creator != "alice" {
		t.Errorf("AcceptID %d should be alice's seek", res.AcceptID)
	}

	// A rematch of a different game doesn't match.
	res, _ = m.Rematch(bob, 7, Request{Creator: "bob", Opponent: "alice", Settings: std5})
	if res.AcceptID != 0 || res.Created.RematchOf != 7 {
		t.Errorf("unrelated rematch: %+v", res)
	}
}

// A third player can't post a rematch of someone else's game and have it
// accepted in place of the real opponent's.
func TestRematchIgnoresOtherCreators(t *testing.T) {
	m := NewManager()
	alice, bob, mallory := &listener{}, &listener{}, &listener{}

	hijack, _ := m.Rematch(mallory, 42, Request{Creator: "mallory", Opponent: "bob", Settings: std5})
	res, err := m.Rematch(bob, 42, Request{Creator: "bob", Opponent: "alice", Settings: std5})
	if err != nil || res.AcceptID != 0 || res.Created.ID == 0 {
		t.Fatalf("bob should post his own rematch, not accept mallory's: %+v, %v", res, err)
	}

	// Alice, the real opponent, still matches bob's seek.
	res, _ = m.Rematch(alice, 42, Request{Creator: "alice", Opponent: "bob", Settings: std5})
	if s, ok := m.Get(res.AcceptID); !ok || s.Creator != "bob" {
		t.Errorf("alice should be told to accept bob's seek, got %+v", res)
	}
	if _, ok := m.Get(hijack.Created.ID); !ok {
		t.Error("mallory's seek is a plain private seek and should stay open")
	}
}
