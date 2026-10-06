package game

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/player"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/tak"
)

// ErrCantAccept means the seek can't be accepted: it's gone, it's the
// acceptor's own, it's addressed to someone else, the acceptor's client
// can't display it, or one of the players is already in a game.
var ErrCantAccept = errors.New("seek can't be accepted")

type Config struct {
	ReconnectTimeout           time.Duration
	TournamentReconnectTimeout time.Duration
	// CommandTimeout bounds how long a session waits for a game to answer.
	CommandTimeout time.Duration
	// AllowMultipleSessions relaxes the rules that keep a player's single
	// connection either playing or observing, never both.
	AllowMultipleSessions bool
}

type Store interface {
	Insert(ctx context.Context, r games.Row) (int, error)
}

type Ratings interface {
	Rating(ctx context.Context, playerID int, at time.Time) (int, error)
}

type ResultSaver interface {
	Save(r games.Result) error
}

// timer is a pending call scheduled by afterFunc.
type timer interface{ Stop() bool }

// Listener is a session that receives game list updates.
type Listener interface {
	Send(ev protocol.Event)
	ProtocolVersion() int
}

// Manager tracks games in progress and which players are in them, and keeps
// listeners' game lists up to date. A player is in at most one game.
type Manager struct {
	cfg     Config
	store   Store
	ratings Ratings
	results ResultSaver
	seeks   *seek.Manager
	players *player.Registry
	log     *slog.Logger
	now     func() time.Time
	// afterFunc runs f on its own goroutine after d; tests replace it.
	afterFunc func(d time.Duration, f func()) timer

	mu        sync.Mutex
	games     map[int]*Game
	byPlayer  map[string]*Game // by name; a nil value reserves the player while a game is created
	listeners map[Listener]struct{}
	observing map[Listener]map[*Game]struct{} // the games each session observes
}

func NewManager(cfg Config, store Store, ratings Ratings, results ResultSaver, seeks *seek.Manager, players *player.Registry, log *slog.Logger) *Manager {
	return &Manager{
		cfg: cfg, store: store, ratings: ratings, results: results, seeks: seeks, players: players, log: log,
		now:       time.Now,
		afterFunc: func(d time.Duration, f func()) timer { return time.AfterFunc(d, f) },
		games:     make(map[int]*Game),
		byPlayer:  make(map[string]*Game),
		listeners: make(map[Listener]struct{}),
		observing: make(map[Listener]map[*Game]struct{}),
	}
}

// Subscribe starts sending game list updates to l, beginning with every
// current game it can display.
func (m *Manager) Subscribe(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners[l] = struct{}{}
	m.sendListLocked(l)
}

func (m *Manager) Unsubscribe(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.listeners, l)
}

// SendList sends l every current game it can display.
func (m *Manager) SendList(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendListLocked(l)
}

// ForPlayer returns the game the named player is in, or nil.
func (m *Manager) ForPlayer(name string) *Game {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byPlayer[name]
}

// Forfeit ends the named player's game, if any, as a loss for them, e.g.
// when they're banned. It reports whether there was a game.
func (m *Manager) Forfeit(ctx context.Context, name string) (bool, error) {
	g := m.ForPlayer(name)
	if g == nil {
		return false, nil
	}
	if err := g.Resign(ctx, name); err != nil && !errors.Is(err, ErrNotPlaying) {
		return true, err
	}
	return true, nil
}

// IfIdle runs f, and reports true, only if the named player isn't in or
// joining a game. f runs while games can't start, so a player can't post a
// seek at the moment they enter a game.
func (m *Manager) IfIdle(name string, f func()) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, busy := m.byPlayer[name]; busy {
		return false
	}
	f()
	return true
}

// Accept starts a game from seek seekID for acceptor, who sent the command
// from session from.
func (m *Manager) Accept(ctx context.Context, acceptor *player.Player, from Listener, seekID int) (*Game, error) {
	// Take the seek and reserve both players atomically, without touching
	// the database, so no seek is accepted twice and no player ends up in
	// two games.
	m.mu.Lock()
	sk, ok := m.seeks.Get(seekID)
	var creator *player.Player
	if ok {
		creator = m.players.Lookup(sk.Creator)
	}
	_, acceptorBusy := m.byPlayer[acceptor.Name]
	if !ok || creator == nil || acceptorBusy || seek.Listener(from) == sk.Owner ||
		from.ProtocolVersion() < sk.RequiredProtocolVersion() ||
		(sk.Opponent != "" && !strings.EqualFold(sk.Opponent, acceptor.Name)) {
		m.mu.Unlock()
		return nil, ErrCantAccept
	}
	if _, busy := m.byPlayer[creator.Name]; busy {
		m.mu.Unlock()
		return nil, ErrCantAccept
	}
	m.seeks.Remove(seekID)
	var stopObserving []Listener
	for _, p := range []*player.Player{acceptor, creator} {
		for _, c := range p.Clients() {
			m.seeks.Cancel(c)
			// single-session: a player's only connection stops observing
			// when they start playing; the client shows a single board.
			if l, ok := c.(Listener); ok && !m.cfg.AllowMultipleSessions {
				stopObserving = append(stopObserving, l)
			}
		}
		m.byPlayer[p.Name] = nil
	}
	m.mu.Unlock()
	for _, l := range stopObserving {
		m.UnobserveAll(l)
	}

	g, err := m.create(ctx, sk, acceptor, creator)
	if err != nil {
		m.mu.Lock()
		delete(m.byPlayer, acceptor.Name)
		delete(m.byPlayer, creator.Name)
		m.mu.Unlock()
		// The seek is already gone, so both sides are told why.
		msg := protocol.ServerMessage{Text: "Could not start the game. Please try again."}
		from.Send(msg)
		if sk.Owner != seek.Listener(from) {
			sk.Owner.Send(msg)
		}
		return nil, err
	}

	m.mu.Lock()
	m.games[g.ID()] = g
	m.byPlayer[acceptor.Name] = g
	m.byPlayer[creator.Name] = g
	m.broadcastLocked(protocol.GameListAdded{Game: g.info})
	m.mu.Unlock()

	g.send(tak.White, protocol.GameStarted{Game: g.info, Color: protocol.White})
	g.send(tak.Black, protocol.GameStarted{Game: g.info, Color: protocol.Black})
	g.start()
	go g.run()
	return g, nil
}

// create records the game in the database and builds it.
func (m *Manager) create(ctx context.Context, sk seek.Seek, acceptor, creator *player.Player) (*Game, error) {
	white, black := acceptor, creator
	switch sk.Color {
	case protocol.White:
		white, black = creator, acceptor
	case protocol.AnyColor:
		if rand.IntN(2) == 0 {
			white, black = creator, acceptor
		}
	}
	start := m.now()
	ratingWhite, err := m.ratings.Rating(ctx, white.ID, start)
	if err != nil {
		return nil, err
	}
	ratingBlack, err := m.ratings.Rating(ctx, black.ID, start)
	if err != nil {
		return nil, err
	}
	s := sk.GameSettings
	id, err := m.store.Insert(ctx, games.Row{
		Date: start, Size: s.Size, White: white.Name, Black: black.Name, Time: s.Time, Increment: s.Increment,
		RatingWhite: ratingWhite, RatingBlack: ratingBlack, Unrated: s.Unrated, Tournament: s.Tournament,
		Komi: s.Komi, Pieces: s.Pieces, Capstones: s.Capstones, ExtraTimeAmount: s.ExtraTimeAmount,
		ExtraTimeTrigger: s.ExtraTimeTrigger, IncrementScales: s.IncrementScales, Opening: s.Opening.PTN(),
	})
	if err != nil {
		m.log.Error("could not record new game", "white", white.Name, "black", black.Name, "err", err)
		return nil, err
	}
	info := protocol.GameInfo{ID: id, White: white.Name, Black: black.Name, Bot: white.IsBot() || black.IsBot(), GameSettings: s}
	m.log.Info("game started", "game", id, "white", white.Name, "black", black.Name)
	return newGame(m, info, white, black), nil
}

// finished removes a game that has ended and saves its result. It runs on
// the game's goroutine.
func (m *Manager) finished(g *Game, notation string, r tak.Result) {
	m.mu.Lock()
	delete(m.games, g.ID())
	for _, s := range g.seats {
		if m.byPlayer[s.name] == g {
			delete(m.byPlayer, s.name)
		}
	}
	m.broadcastLocked(protocol.GameListRemoved{Game: g.info})
	for o := range g.observers {
		m.dropObservingLocked(o, g)
	}
	m.mu.Unlock()

	m.log.Info("game over", "game", g.ID(), "result", r.String())
	if err := m.results.Save(games.Result{ID: g.ID(), Notation: notation, Result: r.String()}); err != nil {
		m.log.Error("could not save game result", "game", g.ID(), "notation", notation, "result", r.String(), "err", err)
	}
}

// Observe starts sending game id's events to session l, after the game so
// far. The named player logged in on l can't observe their own game, a game
// their client can't display, or (single-session) any game while playing.
func (m *Manager) Observe(ctx context.Context, name string, l Listener, id int) error {
	m.mu.Lock()
	g := m.games[id]
	m.mu.Unlock()
	if g == nil {
		return ErrCantObserve
	}
	if err := g.observe(ctx, name, l); err != nil {
		if errors.Is(err, ErrNotPlaying) {
			return ErrCantObserve // it ended meanwhile
		}
		return err
	}
	return nil
}

// Unobserve stops sending game id's events to l. Like the Java server it
// succeeds for any game in progress, observed or not.
func (m *Manager) Unobserve(ctx context.Context, l Listener, id int) error {
	m.mu.Lock()
	g := m.games[id]
	m.mu.Unlock()
	if g == nil {
		return ErrNotPlaying
	}
	return g.do(ctx, func() { g.unobserve(l) })
}

// UnobserveAll stops every observation by l, e.g. when it disconnects.
func (m *Manager) UnobserveAll(l Listener) {
	m.mu.Lock()
	gs := m.observing[l]
	delete(m.observing, l)
	m.mu.Unlock()
	// Without the lock: a game's goroutine may be waiting for it.
	for g := range gs {
		g.post(func() { g.unobserve(l) })
	}
}

// ObservedGames returns the IDs of the games l observes, in order.
func (m *Manager) ObservedGames(l Listener) []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int, 0, len(m.observing[l]))
	for g := range m.observing[l] {
		ids = append(ids, g.ID())
	}
	sort.Ints(ids)
	return ids
}

// addObserver records that l observes g, unless the named player is
// barred because they're in a game. It runs on g's goroutine.
func (m *Manager) addObserver(l Listener, g *Game, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// single-session: a player's only connection can't observe while they
	// play. Checked under the lock so it can't race a game starting.
	if _, playing := m.byPlayer[name]; playing && !m.cfg.AllowMultipleSessions {
		return false
	}
	if m.observing[l] == nil {
		m.observing[l] = make(map[*Game]struct{})
	}
	m.observing[l][g] = struct{}{}
	return true
}

// removeObserver forgets that l observes g. It runs on g's goroutine.
func (m *Manager) removeObserver(l Listener, g *Game) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropObservingLocked(l, g)
}

func (m *Manager) dropObservingLocked(l Listener, g *Game) {
	delete(m.observing[l], g)
	if len(m.observing[l]) == 0 {
		delete(m.observing, l)
	}
}

// broadcastLocked sends ev to every listener that can display the game.
// Protocol versions and game settings never change, so a listener skipped
// for a game's Add is skipped for its Remove too.
func (m *Manager) broadcastLocked(ev protocol.Event) {
	var required int
	switch ev := ev.(type) {
	case protocol.GameListAdded:
		required = ev.Game.RequiredProtocolVersion()
	case protocol.GameListRemoved:
		required = ev.Game.RequiredProtocolVersion()
	}
	for l := range m.listeners {
		if l.ProtocolVersion() >= required {
			l.Send(ev)
		}
	}
}

func (m *Manager) sendListLocked(l Listener) {
	ids := make([]int, 0, len(m.games))
	for id := range m.games {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		info := m.games[id].info
		if l.ProtocolVersion() >= info.RequiredProtocolVersion() {
			l.Send(protocol.GameListAdded{Game: info})
		}
	}
}
