# Go Migration Plan

## 1. Current Architecture Analysis

Before planning the target state, here's what we're actually working with.

### Key observations from the code

- **`Websocket.java`**: hand-rolled WebSocket implementation (raw TCP, manual
  frame parsing, ISO-8859-1 encoding).
- **`Telnet.java`** extends `Websocket`: shares the base class but overrides
  everything. It is just newline-delimited raw TCP.
- **`Client.java`**: one thread per connection, blocking I/O, and a giant regex
  if/else chain for command parsing.
- **`Game.java`**: `ReentrantLock` everywhere, `Timer` for clock management,
  tightly coupled to `Player`.
- **`Player.java`**: auth, session and DB access all in one object, with one
  connection per player.
- No separation between transport, protocol and business logic layers.
- SQLite with direct JDBC, and no connection pooling.

### Current limitations being addressed

- One thread per connection won't scale well past a few hundred connections.
- `Player` is tied to a single `Client`, so a player can't play multiple games.
- No async game support.
- Protocol parsing is fragile positional regex with no versioning abstraction.

## 2. Target Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                         │
│  ┌─────────────────┐          ┌──────────────────────────┐  │
│  │  WebSocket      │          │  Telnet                  │  │
│  │  (gorilla/ws)   │          │  (raw TCP + newline)     │  │
│  └────────┬────────┘          └──────────┬───────────────┘  │
│           │                              │                  │
│           └──────────┬───────────────────┘                  │
│                      ▼                                      │
│            ┌──────────────────┐                             │
│            │  Connection Hub  │  (manages all connections)  │
│            └────────┬─────────┘                             │
└─────────────────────┼───────────────────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────────────────┐
│                Protocol Layer                               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Message Router                                       │  │
│  │  ┌──────────────────┐   ┌────────────────────────┐    │  │
│  │  │ Legacy Text      │   │ JSON Protocol (new)    │    │  │
│  │  │ Parser/Formatter │   │ Parser/Formatter       │    │  │
│  │  └──────────────────┘   └────────────────────────┘    │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────────────────┐
│                 Service Layer                               │
│  ┌────────────┐  ┌─────────────┐  ┌──────────────────────┐  │
│  │  Auth      │  │  Game       │  │  Chat / Rooms        │  │
│  │  Service   │  │  Service    │  │  Service             │  │
│  └────────────┘  └─────────────┘  └──────────────────────┘  │
│  ┌────────────┐  ┌─────────────┐  ┌──────────────────────┐  │
│  │  Seek      │  │  Player     │  │  Admin               │  │
│  │  Service   │  │  Service    │  │  Service             │  │
│  └────────────┘  └─────────────┘  └──────────────────────┘  │
│  ┌────────────────────────────────────────────────────┐     │
│  │  Integrity Service (anti-cheat, tournament games)  │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────────────────┐
│                 Data Layer                                  │
│  ┌────────────────────────┐  ┌──────────────────────────┐   │
│  │  SQLite (players.db)   │  │  SQLite (games.db)       │   │
│  │  via sqlc + migrate    │  │  via sqlc + migrate      │   │
│  └────────────────────────┘  └──────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────────────────┐
│               External                                      │
│  ┌─────────────────┐  ┌───────────────────────────────────┐ │
│  │  HTTP REST API  │  │  GameUpdateBroadcaster (webhook)  │ │
│  │  (seeks, auth,  │  │                                   │ │
│  │   etc.)         │  │                                   │ │
│  └─────────────────┘  └───────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 3. Project Structure

The tree as built for phase 1. Packages marked *planned* don't exist yet.

```
tak-server/
├── cmd/server/
│   └── main.go                # loads config, opens the databases, wires everything, shuts down
├── internal/
│   ├── config/                # settings from environment variables and .env
│   ├── transport/
│   │   ├── connection.go      # Conn, Peer, Acceptor interfaces; send queue; close reasons
│   │   ├── telnet/server.go   # raw TCP, one line per message
│   │   ├── websocket/handler.go # gorilla/websocket upgrade and connection lifecycle
│   │   └── transporttest/     # echo acceptor for transport tests
│   ├── protocol/              # typed commands and events: message.go, settings.go, game.go, chat.go, sudo.go
│   │   ├── legacy/            # text protocol: parser.go, formatter.go, seeks.go, games.go, chat.go, sudo.go
│   │   └── json/              # planned (phase 2): JSON parser and formatter
│   ├── session/
│   │   ├── session.go         # one connection: lifecycle and command dispatch
│   │   ├── manager.go         # every connection, binding logins, presence broadcasts
│   │   ├── account.go         # login, registration, password reset and change
│   │   └── game.go, seek.go, chat.go, sudo.go # handlers for each area
│   ├── auth/                  # service.go (login, register, resets), tokens.go (REST tokens),
│   │                          # password.go (bcrypt), limits.go (rate limits); shared by telnet, WS and REST
│   ├── online/                # who is online: player.go (player and their connections), registry.go
│   ├── seek/manager.go        # open seeks and rematches
│   ├── game/
│   │   ├── game.go            # one goroutine per game: moves, resign, timeouts, sending events
│   │   ├── actions.go         # undo, draw, give time
│   │   ├── observe.go         # observers
│   │   ├── reconnect.go       # disconnects, rejoining, abandoning
│   │   ├── clock.go           # time bookkeeping
│   │   └── manager.go         # starting games, the game list, saving results
│   ├── tak/                   # rules engine: game.go, stone.go, roads.go, tps.go, result.go
│   ├── chat/chat.go           # shout, rooms, tell, masking, gagging
│   ├── moderation/            # sudo commands for moderators and admins
│   ├── wordfilter/            # profanity word list
│   ├── api/                   # REST: http.go (routing, auth endpoints), seeks.go
│   ├── clientip/              # client IP behind a trusted reverse proxy
│   ├── ratelimit/             # per-key token buckets
│   ├── email/                 # SMTP, or logging when no SMTP server is set
│   ├── db/
│   │   ├── sqlite.go          # open, transactions, busy retry
│   │   ├── players/           # store.go (players), tokens.go (auth_tokens)
│   │   ├── games/             # store.go (games), results.go (pending-results file and writer)
│   │   └── dbtest/            # throwaway databases for tests
│   ├── integrity/             # planned (phase 5): anti-cheat, see 4.7
│   └── broadcast/             # planned (later): GameUpdateBroadcaster webhook
├── deploy/                    # deploy.sh, rollback.sh
├── docs/
├── .env.example
├── go.mod
└── go.sum
```

## 4. Critical Design Decisions

### 4.1 Concurrency Model: Goroutines over Threads

The Java server is one thread per connection and guards each game with a
`ReentrantLock`. The Go server uses:

```
Per connection:
  - 1 goroutine reading from the connection
  - 1 goroutine writing to the connection
  - writes go through a buffered queue (1024 lines); sends never block, and
    a client that falls that far behind is disconnected

Per game:
  - 1 goroutine that owns all of the game's state: board, clocks, timers,
    offers, observers. No lock guards it.
  - other goroutines (sessions) send it a command and wait for the reply, up
    to TAK_GAME_COMMAND_TIMEOUT. A command that times out before running is
    cancelled, so it can't take effect later.
  - timers (time.AfterFunc: running out of time, reconnect windows, the bot
    first-move rule) post their work to the same goroutine rather than
    touching the state themselves.
```

Shared registries (online players, seeks, the game list, sessions) are small
maps behind a `sync.Mutex`, held only briefly.

This is the "share memory by communicating" pattern, chosen over a mutex
per game because async games (4.5) will keep many games alive at once, and
one owner per game makes their ordering simple to reason about. It
comfortably handles thousands of concurrent connections.

### 4.2 Backwards Compatibility: The Protocol Layer is the Key

The `Connection` interface will be protocol-agnostic internally. The
protocol is chosen when the connection opens, as the JSON protocol spec
(`json-protocol.md`) defines:

```
WebSocket connection arrives
  Subprotocol "playtak.json.v1", or the query "?protocol=json&v=1":
    use the JSON protocol handler.
  Otherwise, use the legacy text protocol handler.

Telnet connection arrives
  Always starts in the legacy text protocol. The text command
  "Protocol json" (before login) switches it to JSON.
```

- A connection uses one protocol for its whole life. It cannot switch back.
- JSON connections use WebSocket **text** frames. Legacy connections keep
  using **binary** frames, as the Java server does, so existing clients do
  not break. The frame type is chosen per connection.
- JSON connections get one WebSocket close code per close reason (`1001`,
  `1009`, `1011`, `1012`, and `4000` to `4008`). Legacy connections keep
  today's close codes.

The internal message types are the source of truth. Both parsers convert to
and from them. The legacy formatter must produce byte-for-byte identical
output to the Java server for all existing message types. The legacy
protocol is feature-frozen: new features go only to JSON.

### 4.3 Player vs Session Separation

This directly addresses the "multiple games per user" requirement:

```
Java:  Player <-> Client  (1:1 hard coupling)

Go:    Player (identity, DB record, game associations)
          ^
       Session (one per connection, holds protocol version,
                points to Player if authenticated)
          ^
       Connection (transport, send channel)
```

A player can have multiple active sessions (JSON protocol only).

- **Games belong to the player, not to a session.** A player can play any
  number of games at once. A session gets a game's events only once it is
  attached to the game (`game.attach`, `game.detach`), and any attached
  session can play in it.
- The server never attaches a session by itself. When a player's game
  starts or ends, every session of theirs gets `session.games`.
- A player is **away** from a game when no session of theirs is attached to
  it. That starts the reconnect window.
- Seeks also belong to the player. They are removed when the player's last
  session closes.
- Each session independently tracks the games it is attached to or
  observing, its chat rooms, its protocol version and its auth state.
- **JSON takes precedence over text.** While a player has a JSON session
  open, a text login is refused. A JSON login closes the player's text
  session. Text sessions stay single-session.

**Tournament exception:** while a player is in a tournament game, they have
only one session:

- A new session **replaces** the old ones, which are closed with the
  `replaced` reason. A session that died without closing therefore never
  locks the player out while their clock runs.
- If a player has several sessions when the tournament game starts, each
  gets `session.choose_connection`. The one that sends
  `session.keep_connection` stays. If none answers within 30 seconds, the
  most recently active one stays.
- During the game the player cannot start another game or observe one.
  A tournament game cannot start while either player has another game in
  progress, and starting one removes both players' other seeks.

This is an application-layer control, not a network-layer one (see 4.6 for
why IP blocking was not chosen as the main defense). It does not stop a
player from running a second bot game under a different account, device, or
guest session. That case is handled by the integrity service (4.7), not by
session enforcement.

### 4.4 Auth Service: Shared Across Transports

Auth needs to serve three entry points: telnet, WebSocket, and a new HTTP
REST API for upcoming apps and features (web client, mobile, etc.). Rather
than duplicating auth logic per transport, `auth.Service` is the single
source of truth:

```
auth.Service.Login(username, password) -> (Player, error)
auth.Service.Register(...)             -> (Player, error)
auth.Service.Guest()                   -> (Player, error)
```

- Legacy text connections (telnet and WebSocket) call it inline from the
  legacy protocol parser, with a password, same as today.
- The JSON protocol **never accepts a password**, on any transport.
  `session.login` takes exactly one of an access token, a guest token or a
  bot's API key.
- REST (`internal/api/http.go`) exposes the versioned endpoints under
  `/api/v1/`: `auth/login`, `auth/register`, `auth/refresh`, `auth/guest`,
  `auth/logout`, `auth/reset/request`, `auth/reset/confirm` and
  `auth/password`. Every REST request must send an `X-Client-Name` header,
  and CORS is open to any website.

**Token strategy:** use one shared token issued by `auth.Service`, no matter
which entry point the player used to log in. A web or mobile client logs in
over REST, gets a token, then opens a WebSocket and sends the token in
`session.login`, after `session.init`. The token is checked once, at login.
This reuses the Session <-> Player relationship in 4.3 instead of running a
separate REST-only token scheme (like a standalone JWT setup) alongside the
connection-based session scheme.

- Access tokens last 1 hour, refresh tokens 30 days, rotated on refresh.
- Guest tokens are valid for 8 hours after the guest's last activity.
- **Bot API keys:** each bot account has one owner and at most one key. The
  owner, a moderator or an admin creates, replaces or revokes it through
  `/api/v1/bots/{name}/api-key`. Only a hash is stored. Replacing or
  revoking a key closes the sessions that used it.
- Setting another player's password moves to REST
  (`/api/v1/admin/users/{name}/password`), because the JSON protocol never
  carries passwords.

### 4.5 Async Games

The current model assumes both players are online at the same time. Async
support requires:

- Games stored with full state in the DB (not just notation, but current
  board and clock state)
- Clock only ticks when the current player is connected (or this is
  configurable)
- Notifications (email or webhook) when it's a player's turn
- Game resumption on reconnect, loading full state from the DB

- Async games do not count as games in progress for the tournament rule. A
  player in a tournament game cannot move in their async games, and their
  move deadlines are paused until the tournament game ends.
- Untimed seeks (`time` 0) are only allowed once async games exist.

This is a later feature. The JSON protocol spec lists it under Planned. The
architecture should allow for it, but the initial port does not need to
implement it.

### 4.6 Tournament Anti-Cheat: Why Not IP Blocking

The original idea was to store a tournament player's IP and block any other
connection from that IP. On review, this has problems:

- **False positives.** Campus networks, offices, shared households, and
  mobile carrier CGNAT routinely put many legitimate players behind one IP.
  Blocking on this basis blocks innocent players too.
- **Easy to bypass.** VPN or proxy rotation defeats an IP check entirely. The
  players most motivated to cheat are also the most likely to route around
  it.
- **IPv6 makes it worse.** Many ISPs assign a large address block per
  household, so "same IP" is not even a stable idea there.

What the requirement actually needs is identity control: one player, one
session, per tournament game. That is a session problem, covered in 4.3.
IP is kept as metadata only, logged alongside session and game activity for
admin review and as one weighted input to the integrity scoring in 4.7. It is
never used to auto-block.

### 4.7 Integrity Service: Move-Correlation Anti-Cheat

**The problem:** a player could run a tournament game on their
session-limited account while separately running a parallel game against one
of the community bots, under a different account, browser, device, or guest
session that session limiting cannot see or stop, and copy the bot's moves
into the tournament game.

**Why a live-engine-query approach does not work here:** the 15 bot levels
are independently coded and hosted by community members. We do not own them
and cannot cheaply ask them "what would you play in this position" on
demand. Running deep searches against all 15, or even just the strongest
one, on every move is not realistic on a 2 core, 4GB VPS.

**Design:** passive, event-driven, self-pruning correlation against live bot
games.

- **Board hashing (`boardhash.go`).** Every position is normalized for Tak's
  8-fold symmetry (rotations and reflections) into one canonical hash.
  Without this, mirrored or rotated versions of the same strategic position
  would look like different positions and matches would be missed.
- **Live position index (`liveindex.go`).** A small in-memory map of
  `position_hash -> [active bot game IDs]`, built and updated as
  currently-open bot games progress. Entries are added as bot games advance
  and removed as bot games end. This is a cheap, constantly-changing lookup
  table sized to current bot game traffic, not a query against a large
  historical archive.
- **Tournament watch (`tourneywatch.go`).** Runs only while a tournament game
  is in progress, which is a small share of total traffic, so cost stays
  proportional to tournament load:
  - On each tournament move, hash the position and look it up in the live
    index.
  - Keep a small set of candidate active bot games whose position sequence
    still matches the tournament game's sequence, move for move.
  - Early moves (openings) are not scored. Shared openings will match a large
    number of active bot games by coincidence and carry no real signal.
  - Each later move either extends a candidate's consecutive-match streak or
    breaks it. A candidate that builds up 2 to 3 consecutive deviating moves
    is dropped from tracking. No more time is spent comparing against it.
  - The suspicion contribution from a candidate is its streak length at the
    time of comparison, not a running point total. A long streak that
    survives deep into a game, past the point where coincidence is
    plausible, is strong evidence. A streak that broke early contributed
    almost nothing and is simply dropped, not decayed.
  - This produces a shrinking, self-pruning set of candidates by design:
    many plausible matches early on, narrowing to zero or one quickly. That
    fits the limited VPS resources well.
- **IP correlation (`ipcorrelate.go`).** Kept as metadata only (4.6),
  contributing as one weighted input rather than a trigger on its own.
- **Scoring (`scoring.go`).** Combines tourneywatch streaks, reference engine
  matches (once available, see below), and IP overlap into one suspicion
  score. No single signal auto-flags or auto-bans a player.
- **Review queue (`queue.go`).** Flagged games and players go to the admin
  service's review queue for a person to judge. This system is inherently
  probabilistic. False positives, such as a genuinely strong player or
  someone who studied a known opening line, are expected and should never
  trigger automatic enforcement.

**Known limitation of the live-index approach:** it can only catch relaying
when some active bot game happens to be shadowing the tournament game's
positions at that moment. A player relaying against a novel or rare
position, with no matching active bot game to catch it, slips through. This
gap is the reason for the reference engine addition below.

#### Future addition: a small, self-hosted reference engine

This is for a later phase, not part of the initial build.

The community bots (15 total) are independently coded and hosted by
community members, so querying them live on demand is not realistic, as
covered above. A better long-term fix for the coverage gap is a single
lightweight engine that we host ourselves, used only to generate a ranked
list of likely moves for a given position on request:

- Run at a fixed, shallow search depth (depth 4 to 6 is enough to produce a
  reasonable ranked move list, not grandmaster strength) so it stays cheap
  on a 2 core, 4GB VPS.
- Only queried for tournament moves, not every game on the server, keeping
  cost proportional to tournament load.
- Rather than writing a search algorithm from scratch, an existing
  open-source Tak engine could be adapted and run as a local subprocess at a
  capped depth/time budget. Several exist, including at least one considered
  the strongest public Tak bot, built on Monte Carlo tree search, and it
  already speaks a UCI-style protocol suited to this kind of on-demand
  querying.
- Results should be cached by position hash, so a repeated position across
  games does not trigger a repeated search.
- The human's move is checked against the engine's top-N ranked candidates
  (matched the top pick, in the top 3, or neither), and the same
  streak-based scoring logic used for `tourneywatch.go` applies here too:
  early moves are not scored, and a sustained streak of top-pick matches
  across a game is the real signal, not a single match.
- Worth deciding later whether the search depth should scale with the
  tournament's skill tier. Matching a strong engine's top pick consistently
  in a beginner tournament is more suspicious than in an advanced one, since
  a beginner should not naturally be finding engine-strength moves.

This is intentionally scoped as a later addition. It requires standing up
and hosting an engine, which is more work than the live-index check, and the
live-index check already provides real coverage from day one with no extra
infrastructure. The two are complementary: the live check catches relaying
against currently active bot games with no extra cost, and the reference
engine (once built) extends that same kind of check to any position, closing
the gap the live check cannot.

## 5. Migration Phases

### Phase 1: Foundation (No Feature Changes)

**Goal:** the Go server passes all existing client compatibility tests.

- [ ] Project setup, config loading, logging
- [ ] WebSocket transport (gorilla/ws), must be wire-compatible with the
      current server
- [ ] Telnet transport, newline-delimited, identical behavior
- [ ] Legacy protocol parser, port all regexes from `Client.java`
- [ ] Legacy protocol formatter, produce identical output strings
- [ ] In-memory session management
- [ ] Auth service (BCrypt, login, guest, register, password reset), shared
      by telnet/ws and REST (4.4)
- [ ] HTTP auth endpoints (`/api/v1/auth/*`) alongside telnet/ws auth
- [ ] Player registry
- [ ] Seek manager (V1-V4, rematch)
- [ ] Game engine (port `Game.java` and the `Board` inner class)
  - Board logic
  - Clock logic (`clock.go`)
  - Move validation (place and move)
  - Win conditions (road, flat, draw)
  - Undo, draw offer, resign, give time
- [ ] Chat (shout, rooms, tell)
- [ ] Admin/sudo commands
- [ ] DB layer (sqlc queries matching the existing schema exactly)
- [ ] GameUpdateBroadcaster webhook
- [ ] HTTP REST API (seeks endpoints)

**Compatibility test suite**, built alongside Phase 1:

- Telnet script that replays the full protocol flow
- Compare output byte-for-byte against the Java server for every message
  type and version

### Phase 2: New Capabilities

**Goal:** new features without breaking Phase 1.

- [ ] JSON protocol handler (parser and formatter), as defined in
      `json-protocol.md`
- [ ] Protocol negotiation on connect (subprotocol, query parameter,
      `Protocol json` on telnet)
- [ ] Text frames for JSON, binary frames kept for legacy
- [ ] JSON close codes and `session.closed` reasons, including
      `not_reading` and `send_queue_full`
- [ ] Multiple sessions per player, with games and seeks owned by the
      player, and `game.attach` / `game.detach`
- [ ] JSON sessions take precedence over text sessions
- [ ] Tournament rules (4.3): one session per player, new replaces old,
      `session.choose_connection`, no other games or observing
- [ ] `session.online` sent at most once a second
- [ ] Seek validation on JSON: reject out-of-range values with
      `INVALID_FIELD` (legacy keeps clamping)
- [ ] New REST endpoints: password reset and change, bot API keys, bot
      owner, admin set password, `X-Client-Name`, CORS
- [ ] Async game mode (later; see 4.5)
- [ ] DB migrations for new columns
- [ ] Proper connection pool for SQLite (or migrate to PostgreSQL)
- [ ] Integrity service v1: board hashing, live position index, tournament
      watch, scoring, admin review queue (4.7)

### Phase 3: Operational

- [ ] Structured logging (slog)
- [ ] Metrics (New Relic)
- [ ] Docker image
- [ ] Graceful shutdown (port the sigterm/message file behavior)
- [ ] Load testing
- [ ] Reference engine integrity signal: adapt/host a lightweight local Tak
      engine, ranked move queries, streak scoring, feed into 4.7 scoring
      (future phase, closes the live-index coverage gap)

## 6. Phase 2 Implementation Plan

This is what has to change in the Go server for the JSON protocol, checked
against the current code. `json-protocol.md` is the source of truth. Where
this section and the spec disagree, the spec wins.

Most of the work is refactoring the existing layers so JSON can sit beside
the text protocol. The rest is new JSON code.

Two rules apply to all of it:

- **Text-protocol output stays byte-identical.** The compatibility suite
  must keep passing (153 tests, 0 failures) after every step.
- **The games database keeps storing moves in the current format**
  (`P E5`, `M A1 A3 1 2`), because the NestJS rating job and the game
  history API read it.

### 6.1 Refactors to existing code, with no change in behavior

| # | Change | Why | Where |
| --- | --- | --- | --- |
| 1 | **A swappable format per connection.** The session calls `legacy.Parse` and `legacy.Format` directly. Put both behind an interface the session holds, and make the text protocol one implementation of it. | Each connection must speak either format, and telnet switches partway through | `session/session.go` |
| 2 | **Results for replies.** Handlers send `OK`, `NOK` or nothing in many different ways. Have each handler return a result instead: success, or an error code. The text format renders exactly what it sends today, and JSON renders `ack` or `error` with the `id`. | Every JSON command gets exactly one reply, matched by `id` | every handler in `session/` |
| 3 | **Stable error codes.** Map internal errors to the spec's codes, for example `game.ErrNotPlaying` to `NOT_IN_GAME`, `auth.ErrRateLimited` to `RATE_LIMITED`, and an illegal move to `ILLEGAL_MOVE`. | The JSON `error` event | `protocol/` |
| 4 | **The reply must arrive before the events.** Today the game goroutine sends `Time` and the move event while the command is still running, and the reply comes after. Either the game sends the reply from its goroutine before the events, or the session holds back its own events until the reply is out. | Spec rule 6 | `game/` (`game.go`, `actions.go`, `reconnect.go`), `session/` |
| 5 | **Compatibility levels instead of version numbers.** About 16 places check `ProtocolVersion()`. Replace them with a level: text 0 to 3 is below the baseline, text 4 equals JSON 1.0, and JSON 1.x is 1.x. `RequiredProtocolVersion` becomes the minimum version a seek or game needs. | Version gating, and players on both formats playing each other | `seek`, `game`, `api`, `player`, `session` |
| 6 | **Moves as data, not strings.** The engine records moves as strings such as `P A1 W`. Store a move structure instead and format it on the way out: the text format for the wire and the database, PTN for JSON. This needs a new PTN parser (see 6.3). | PTN only in JSON | `tak/game.go`, new `tak/ptn.go` |
| 7 | **A full-state game event.** Have the game send one state event (TPS, ply, what changed, turn, clocks, offers, result) next to today's events. The text format ignores it, and JSON ignores `Time` and the move event. Add a snapshot event for `game.start`. | `game.start` and `game.state` | `game/`, `protocol/game.go` |
| 8 | **An explicit end reason.** Game over is a result string plus a separate quitter name today. Use a reason (`road`, `flats`, `resign`, `time`, `abandon`, `agreement`, `no_progress`, `ban`) and the winner's username. Flat ties and the 50-ply no-progress draw must be told apart. | `reason` and `winner` fields | `game/` (`game.go`, `reconnect.go`), `tak/` |
| 9 | **Structured moderation results.** `moderation.Result` holds text lines. The `admin.*` acks need data, such as `usernames` for `admin.list` and `changed` for `admin.set_role`, as well as text. | `admin.*` acks | `moderation/` |

### 6.2 Transport changes

| Change | Where |
| --- | --- |
| WebSocket: accept the subprotocol `playtak.json.v1` and the query `?protocol=json&v=1`. | `transport/websocket/handler.go` |
| WebSocket: for JSON, send text frames with no trailing newline. Today every frame is binary and ends in `\n`. Legacy connections keep binary frames. | same |
| WebSocket: turn on `permessage-deflate` (`EnableCompression` in gorilla, no new dependency). Accept any `Origin`. | same |
| WebSocket: send the spec's close code for each close reason on JSON connections. Legacy keeps today's codes. | same, `transport/connection.go` |
| Tell the session which format a connection uses when it is accepted. A JSON WebSocket must not get the `Welcome!` greeting. | transport, `session.Manager.Accept` |
| Telnet: `Protocol json` switches the connection. The parser accepts digits only today (`^Protocol ([0-9]{1,8})`). | `protocol/legacy/parser.go`, `session/` |
| Slow connections: a 10-second stuck write closes with `not_reading`. A full send queue drops the waiting messages, sends `session.closed` with `send_queue_full`, then closes. | `transport/` |

### 6.3 New code

- **`internal/protocol/json`:** decoding commands and encoding events.
  Unknown fields are allowed. A missing or wrong field gives
  `INVALID_FIELD` naming it, and bad JSON gives `INVALID_JSON` with no `id`.
  Every event carries `ts`.
- **Session commands:** `session.init` (version check, `client_name`),
  `session.login` (access token through the existing `auth.Authenticate`,
  guest token, or bot API key), `session.ping`, the command-order errors
  (`INIT_REQUIRED`, `ALREADY_INITIALIZED`, `NOT_AUTHENTICATED`,
  `ALREADY_LOGGED_IN`), and `session.closed` before every close.
- **PTN:** the server always sends the full form (`1a1>1`, `3c3>12`, `*` on
  a wall flatten, no `F`). It accepts the short form too, checks `*` if sent,
  and rejects annotations and wrong case with `INVALID_FIELD`.
- **Per-connection command rate limit:** the existing `ratelimit` package,
  returning `RATE_LIMITED` with `retry_ms`.
- **Lobby:** `session.online` batched to at most once a second. Seeks with
  an `opponent` go only to the poster and that opponent. Seek ratings
  refreshed hourly, with `seek.updated`.
- **Seek validation on JSON:** reject out-of-range values with
  `INVALID_FIELD` instead of clamping. `one_game_per_opponent`, several
  seeks per player, and rematch seeks (`rematch_of`).
- **Game commands:** `game.decline_draw` and `game.decline_undo` are new,
  with `NO_OPEN_OFFER`.
- **Chat:** the `global` room maps to today's Shout, and `game:<id>` rooms
  map to game rooms. `chat.tell` goes to both players. Room names are 1 to
  64 characters with no spaces, and `global` and `game:` are reserved. New
  `chat.joined` and `chat.left` events.
- **REST:** move to `/api/v1/`, require `X-Client-Name`, open CORS, and add
  the new endpoints: password reset and change, bot API keys, bot owner,
  admin set password. Guest tokens last 8 hours after the last activity.

### 6.4 Games belong to the player, attached per connection

Today a seat belongs to a player name, and `Game.send` reaches every one of
that player's connections. The spec keeps games owned by the player, but a
connection only gets a game's events once it is attached:

- `Game.send` goes to the connections attached to the game, plus
  observers, not to all of the player's connections.
- New `game.attach` and `game.detach`. Every connection gets
  `session.games` when one of the player's games starts or ends. Nothing is
  attached automatically, so JSON logins do not replay the game the way
  `Session.resumeGame` does for text.
- A player is away when no connection is attached, not only when their last
  connection closes.
- Seeks move from one per session to several per player, removed when the
  last connection closes.
- Ending a game detaches everyone and stops observing.
- JSON connections take precedence over text ones: a text login is refused
  while a JSON connection is open, and a JSON login closes a text one.
- Tournament rules: one connection, a new one replaces the old
  (`replaced`), `session.choose_connection` at game start, no other games or
  observing, and other seeks removed.

This changes the "still player-level" list in
`docs/multi-session-cleanup.md`. Update that file as this work lands.

### 6.5 Decisions

All of the decisions the earlier draft left open have been made. They are
recorded in the spec:

- **Games per connection:** no cap. Games belong to the player, and any
  number can be in progress at once. Tournament games stay exclusive.
- **Komi:** `half_komi`, an integer in half flats (0 to 8).
- **Bot login:** both an access token and an API key.
- **Community review:** still in progress. Field names and rules may still
  change, so build the refactors in 6.1 first, since they do not depend on
  the spec's details.

### 6.6 Suggested order

1. **Refactors only (all of 6.1).** Nothing visible changes, and the full
   compatibility suite must still pass. This is the biggest piece.
2. **The JSON skeleton:** transport negotiation, the telnet switch,
   `session.init`, `session.login`, `session.ping`, errors, close codes, rate
   limiting, and per-message golden tests.
3. **Seeks and the lobby lists.**
4. **Games:** start, state, move, resign, sync, offers and declines, undo,
   give time, observe.
5. **Attaching (6.4):** `game.attach`, `game.detach`, `session.games`,
   precedence over text, and the tournament rules.
6. **Chat and moderation.**
7. **REST changes** (6.3).
8. **Tests where players on both formats share a game.**

## 7. Key Risks & Mitigations

_To be written._

## 8. Recommended Libraries

_To be written._

## 9. Suggested Starting Point

Start with the transport and protocol layer before touching game logic. The
path of least resistance:

1. Get a WebSocket connection to say "Welcome!" and "Login or Register".
2. Get login working end-to-end (text protocol), and stub the REST auth
   endpoints against the same `auth.Service`.
3. Get seeks working.
4. Get a game to start.
5. Get moves working, and win detection.
6. Get everything else working (chat, admin, reconnect, etc.).

This validates the most critical backwards-compatibility path early and
often. The integrity service (4.7) and tournament session enforcement (4.3)
are Phase 2 items and should not block this critical path. The auth service
should still be designed from the start (4.4) to serve REST cleanly, since
retrofitting it later is more disruptive.
