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
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
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
	Send(event protocol.Event)
	ProtocolVersion() int
}

// Manager tracks games in progress and which players are in them, and keeps
// listeners' game lists up to date. A player is in at most one game.
type Manager struct {
	config  Config
	store   Store
	ratings Ratings
	results ResultSaver
	seeks   *seek.Manager
	players *online.Registry
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

func NewManager(config Config, store Store, ratings Ratings, results ResultSaver, seeks *seek.Manager, players *online.Registry, log *slog.Logger) *Manager {
	return &Manager{
		config: config, store: store, ratings: ratings, results: results, seeks: seeks, players: players, log: log,
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
func (m *Manager) Accept(ctx context.Context, acceptor *online.Player, from Listener, seekID int) (*Game, error) {
	// Take the seek and reserve both players atomically, without touching
	// the database, so no seek is accepted twice and no player ends up in
	// two games.
	m.mu.Lock()
	openSeek, ok := m.seeks.Get(seekID)
	var creator *online.Player
	if ok {
		creator = m.players.Lookup(openSeek.Creator)
	}
	_, acceptorBusy := m.byPlayer[acceptor.Name]
	if !ok || creator == nil || acceptorBusy || seek.Listener(from) == openSeek.Owner ||
		from.ProtocolVersion() < openSeek.RequiredProtocolVersion() ||
		(openSeek.Opponent != "" && !strings.EqualFold(openSeek.Opponent, acceptor.Name)) {
		m.mu.Unlock()
		return nil, ErrCantAccept
	}
	if _, busy := m.byPlayer[creator.Name]; busy {
		m.mu.Unlock()
		return nil, ErrCantAccept
	}
	m.seeks.Remove(seekID)
	var stopObserving []Listener
	for _, player := range []*online.Player{acceptor, creator} {
		for _, c := range player.Clients() {
			m.seeks.Cancel(c)
			// single-session: a player's only connection stops observing
			// when they start playing; the client shows a single board.
			if l, ok := c.(Listener); ok && !m.config.AllowMultipleSessions {
				stopObserving = append(stopObserving, l)
			}
		}
		m.byPlayer[player.Name] = nil
	}
	m.mu.Unlock()
	for _, l := range stopObserving {
		m.UnobserveAll(l)
	}

	game, err := m.create(ctx, openSeek, acceptor, creator)
	if err != nil {
		m.mu.Lock()
		delete(m.byPlayer, acceptor.Name)
		delete(m.byPlayer, creator.Name)
		m.mu.Unlock()
		// The seek is already gone, so both sides are told why.
		message := protocol.ServerMessage{Text: "Could not start the game. Please try again."}
		from.Send(message)
		if openSeek.Owner != seek.Listener(from) {
			openSeek.Owner.Send(message)
		}
		return nil, err
	}

	m.mu.Lock()
	m.games[game.ID()] = game
	m.byPlayer[acceptor.Name] = game
	m.byPlayer[creator.Name] = game
	m.broadcastLocked(protocol.GameListAdded{Game: game.info})
	m.mu.Unlock()

	game.send(tak.White, protocol.GameStarted{Game: game.info, Color: protocol.White})
	game.send(tak.Black, protocol.GameStarted{Game: game.info, Color: protocol.Black})
	game.start()
	go game.run()
	return game, nil
}

// create records the game in the database and builds it.
func (m *Manager) create(ctx context.Context, openSeek seek.Seek, acceptor, creator *online.Player) (*Game, error) {
	white, black := acceptor, creator
	switch openSeek.Color {
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
	settings := openSeek.GameSettings
	id, err := m.store.Insert(ctx, games.Row{
		Date: start, Size: settings.Size, White: white.Name, Black: black.Name, Time: settings.Time, Increment: settings.Increment,
		RatingWhite: ratingWhite, RatingBlack: ratingBlack, Unrated: settings.Unrated, Tournament: settings.Tournament,
		Komi: settings.Komi, Pieces: settings.Pieces, Capstones: settings.Capstones, ExtraTimeAmount: settings.ExtraTimeAmount,
		ExtraTimeTrigger: settings.ExtraTimeTrigger, IncrementScales: settings.IncrementScales, Opening: settings.Opening.PTN(),
	})
	if err != nil {
		m.log.Error("could not record new game", "white", white.Name, "black", black.Name, "err", err)
		return nil, err
	}
	info := protocol.GameInfo{ID: id, White: white.Name, Black: black.Name, Bot: white.IsBot() || black.IsBot(), GameSettings: settings}
	m.log.Info("game started", "game", id, "white", white.Name, "black", black.Name)
	return newGame(m, info, white, black), nil
}

// finished removes a game that has ended and saves its result. It runs on
// the game's goroutine.
func (m *Manager) finished(game *Game, notation string, result tak.Result) {
	m.mu.Lock()
	delete(m.games, game.ID())
	for _, s := range game.seats {
		if m.byPlayer[s.name] == game {
			delete(m.byPlayer, s.name)
		}
	}
	m.broadcastLocked(protocol.GameListRemoved{Game: game.info})
	for o := range game.observers {
		m.dropObservingLocked(o, game)
	}
	m.mu.Unlock()

	m.log.Info("game over", "game", game.ID(), "result", result.String())
	if err := m.results.Save(games.Result{ID: game.ID(), Notation: notation, Result: result.String()}); err != nil {
		m.log.Error("could not save game result", "game", game.ID(), "notation", notation, "result", result.String(), "err", err)
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
func (m *Manager) addObserver(listener Listener, game *Game, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// single-session: a player's only connection can't observe while they
	// play. Checked under the lock so it can't race a game starting.
	if _, playing := m.byPlayer[name]; playing && !m.config.AllowMultipleSessions {
		return false
	}
	if m.observing[listener] == nil {
		m.observing[listener] = make(map[*Game]struct{})
	}
	m.observing[listener][game] = struct{}{}
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

// broadcastLocked sends event to every listener that can display the game.
// Protocol versions and game settings never change, so a listener skipped
// for a game's Add is skipped for its Remove too.
func (m *Manager) broadcastLocked(event protocol.Event) {
	var required int
	switch ev := event.(type) {
	case protocol.GameListAdded:
		required = ev.Game.RequiredProtocolVersion()
	case protocol.GameListRemoved:
		required = ev.Game.RequiredProtocolVersion()
	}
	for l := range m.listeners {
		if l.ProtocolVersion() >= required {
			l.Send(event)
		}
	}
}

func (m *Manager) sendListLocked(listener Listener) {
	ids := make([]int, 0, len(m.games))
	for id := range m.games {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		info := m.games[id].info
		if listener.ProtocolVersion() >= info.RequiredProtocolVersion() {
			listener.Send(protocol.GameListAdded{Game: info})
		}
	}
}
