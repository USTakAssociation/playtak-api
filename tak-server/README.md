# tak-server

The PlayTak game server, rewritten in Go. It replaces the Java server in
`../server`.

**Status:** phase 1 of the migration, which keeps compatibility with the
Java server. It speaks the same text protocol over telnet and WebSocket,
so the current playtak-ui, bots and the compatibility suite in `../tests`
work unchanged, apart from the deliberate changes listed under
[Changes from the Java server](#changes-from-the-java-server). Phase 2 adds
a JSON protocol, multiple sessions per player and async games.

## Contents

- [How it fits together](#how-it-fits-together)
- [Packages](#packages)
- [Running locally](#running-locally)
- [Configuration](#configuration)
- [Testing](#testing)
- [HTTP API](#http-api)
- [Database migrations](#database-migrations)
- [Changes from the Java server](#changes-from-the-java-server)

## How it fits together

```
 telnet :10000 ──┐
                 ├─ transport ─ session ─┬─ auth ─────────── db/players
 websocket :9999 ┘   (lines)   (one per  ├─ seek
                               connection)├─ game ── tak      db/games
                                         ├─ chat
 http :9998 ──── api ─ auth, seek        └─ moderation
```

- **transport** reads and writes lines on a connection. Each connection has
  a reader goroutine, a writer goroutine and a send queue, so a slow client
  never blocks anyone else.
- **session** turns each line into a typed command, calls the service that
  handles it, and renders the resulting events in the client's protocol
  version. The text format lives only in `protocol/legacy`, so a JSON
  protocol can be added beside it.
- **game** runs one goroutine per live game. That goroutine owns the
  board, the clocks and the timers; sessions send it a command and wait
  for the reply, up to `TAK_GAME_COMMAND_TIMEOUT`. Timers (running out of
  time, reconnect windows) post work to the same goroutine, so a game's
  state never needs a lock.
- **tak** is the rules engine. It's pure (no I/O and no clocks) and has been
  checked move for move against the Java engine.
- **players** are loaded from the database when they log in. Only online
  players and guests are held in memory.
- **Finished games** are written to a pending-results file first, then
  saved to the games DB in the background, with retries. A busy database
  or a restart can't lose a result.

## Packages

| Package | What it does |
| --- | --- |
| `cmd/server` | Entry point. Loads config, opens the databases and wires everything together. |
| `internal/config` | Settings from environment variables, with an optional `.env` file. |
| `internal/transport` | The connection interface the session layer uses. `telnet` and `websocket` implement it. |
| `internal/session` | One `Session` per connection: login, command dispatch, cleanup. `Manager` tracks every connection. |
| `internal/protocol` | Typed commands and events, independent of the wire format. |
| `internal/protocol/legacy` | The text protocol: parsing lines into commands and formatting events per protocol version (0–4). |
| `internal/auth` | Login, registration, password resets and REST tokens. Passwords use bcrypt. |
| `internal/player` | The registry of online players and guests. |
| `internal/seek` | Open seeks and rematches. |
| `internal/game` | Live games: the game goroutine, clocks, undo/draw/give time, observers, disconnects, and saving results. |
| `internal/tak` | The rules of Tak: placing, moving, roads, flat wins, TPS. |
| `internal/chat` | Shout, rooms and Tell, with profanity masking and gagging. |
| `internal/moderation` | `sudo` commands for moderators and admins. |
| `internal/wordfilter` | The profanity word list, used for usernames and chat. |
| `internal/api` | The HTTP REST API. |
| `internal/email` | Sends mail over SMTP, or logs it when no SMTP server is set. |
| `internal/db` | Opens SQLite. `players` and `games` hold the queries, and `dbtest` creates throwaway databases for tests. |

## Running locally

You need Go (the version in `go.mod`) and `sqlite3` for the setup scripts.

1. **Create the databases.** From the repo root:

   ```sh
   bash scripts/development/create_databases.sh
   ```

   This creates `playtakdb/players.db` and `playtakdb/games.db` and applies
   every migration.

2. **Add some accounts.** Every account gets the password `password`:

   ```sh
   bash scripts/development/add_user.sh testuser1 playtakdb/players.db
   bash scripts/development/add_user.sh testuser2 playtakdb/players.db
   ```

   For an admin, add a user, then set the flag:

   ```sh
   bash scripts/development/add_user.sh adminuser playtakdb/players.db
   sqlite3 playtakdb/players.db "UPDATE players SET is_admin = 1 WHERE name = 'adminuser'"
   ```

3. **Configure.** In `tak-server/`:

   ```sh
   cp .env.example .env
   ```

   Then point the databases at the files you just created:

   ```sh
   TAK_PLAYERS_DB=../playtakdb/players.db
   TAK_GAMES_DB=../playtakdb/games.db
   ```

4. **Run it:**

   ```sh
   cd tak-server
   go run ./cmd/server
   ```

   Or build a binary:

   ```sh
   go build -o tak-server ./cmd/server
   ./tak-server
   ```

5. **Try it.** Connect with telnet and log in as a guest:

   ```sh
   nc localhost 10000
   Login Guest
   ```

   Or point playtak-ui at `ws://localhost:9999`.

The server stops cleanly on Ctrl-C or SIGTERM, closing connections with a
proper close frame. Any game result not yet saved to the database stays in
the pending-results file and is saved on the next start.

## Configuration

Everything is set with environment variables. A `.env` file in the working
directory is loaded first, and variables already set in the environment
override it. [`.env.example`](.env.example) documents every variable. The
main ones:

| Variable | Default | Purpose |
| --- | --- | --- |
| `TAK_TELNET_ADDR`, `TAK_WS_ADDR`, `TAK_HTTP_ADDR` | `:10000`, `:9999`, `:9998` | Listen addresses. |
| `TAK_PLAYERS_DB`, `TAK_GAMES_DB` | `/playtakdb/...` | SQLite database files. |
| `TAK_RECONNECT_TIMEOUT` | `2m` | How long a disconnected player's game is held open. |
| `TAK_TOURNAMENT_RECONNECT_TIMEOUT` | `15m` | The same, for tournament games. |
| `TAK_GAME_COMMAND_TIMEOUT` | `10s` | How long a command waits for its game before failing with NOK. |
| `TAK_PENDING_RESULTS_FILE` | beside the games DB | Where finished games wait until they're saved. |
| `TAK_ALLOW_MULTIPLE_SESSIONS` | `false` | Let a player stay logged in on several connections. See [docs/multi-session-cleanup.md](docs/multi-session-cleanup.md). |
| `TAK_BADWORDS_URL` | shared Google Sheet | CSV word list for the profanity filter. Must be an http(s) URL. |
| `TAK_SMTP_*` | empty | Outgoing email. Empty means emails are logged instead; their bodies only at `debug` level, since they can hold passwords. |
| `TAK_SERVICE_API_KEY` | empty | Shared secret for service-to-service calls such as `PUT /api/v1/seeks`. Empty disables them. |
| `TAK_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

`TAK_EVENT_SUBSCRIBER_URL` is read, but the game update webhooks it
configures aren't implemented yet.

## Testing

### Unit tests

```sh
cd tak-server
go test -race ./...
```

The tests cover the unhappy paths as well as the happy ones. Tests use throwaway SQLite databases (`internal/db/dbtest`), and the
game tests replace the clock and timers with fakes, so nothing sleeps. The
rules engine in `internal/tak` has 100% coverage.

### Comparing the rules engine with Java

This test plays random games through both the Go and Java engines and
checks that they agree after every move. It needs a JDK and the
built Java server:

```sh
(cd ../server && mvn package)
go test -tags javadiff ./internal/tak/ -run JavaDiff -v
```

`TAK_DIFF_GAMES` sets the number of games (default 500). `TAK_DIFF_SEED`
replays a failure; the seed is printed on every run.

### Compatibility suite

`../tests` holds the integration tests that run over the wire against a live
server, Java or Go. With the server running against a database that has
`testuser1`–`testuser3` and an admin account:

```sh
cd ../tests
TAK_ADMIN_USER=adminuser TAK_ADMIN_PASS=password go test ./suite/ -v -timeout 300s
```

Without the admin variables, the admin tests are skipped. See
[../tests/README.md](../tests/README.md) for more options. Tests that check
old Java behaviour the Go server deliberately changed are commented out
with a note, next to their Go versions.

## HTTP API

| Method and path | Purpose |
| --- | --- |
| `POST /api/auth/login` | Log in with name and password; returns an access token (1 hour) and a refresh token (30 days). |
| `POST /api/auth/register` | Register; the password is emailed. |
| `POST /api/auth/refresh` | Swap a refresh token for a new pair. The old one stops working. |
| `POST /api/auth/guest` | Get a guest name and token. |
| `POST /api/auth/logout` | Revoke tokens. |
| `GET /api/v1/seeks` | List open seeks. |
| `PUT /api/v1/seeks` | Create a seek (service key required). |

Tokens are stored only as SHA-256 hashes in the `auth_tokens` table.
Expired ones are purged hourly. Admins can revoke a player's tokens with
`sudo revoke <name>`.

## Database migrations

Schema changes are dated shell scripts in `../scripts/migrations`. They're
safe to run more than once. `create_databases.sh` runs all of them for a
fresh development setup. For production, run each new script against the
live database before deploying the server that needs it. At startup the
server checks that the players database has had the migrations it relies
on (the `id` primary key and the `auth_tokens` table), and refuses to start
if not, naming the script to run.

## Changes from the Java server

The deliberate changes in client-visible behaviour, for release notes:

**Games**
- With no moves played, `RequestUndo` replies `NOK` alone, not `NOK` then `OK`.
- `GiveTime` before the clock starts replies `NOK` and `Error:The clock has not started`. After a give, the player must wait 15 seconds before giving again; until then they get `Message You can give time again in N seconds` and `NOK`.
- A move that arrives after the mover's time has run out is refused, and the game ends on time. Java played the move, then ended the game.
- A bot playing black wins if white doesn't move within the starting time or 5 minutes, whichever is shorter. Java always waited 5 minutes.
- A disconnect notice gives the smaller of the reconnect window and the player's remaining clock, if it's their turn.
- On reconnect, the opponent's pending draw offer or undo request is sent again.
- Players can't observe their own game. In single-session mode they can't observe while playing.

**Chat**
- `Tell` only reaches online players. Anyone else, including unknown names, gets `Error:Player is not online`. Names match ignoring case.
- `ShoutRoom` requires having joined the room first, otherwise it replies `Error:You are not in that room`.
- Message length counts characters, so an emoji counts as one, not two.
- The IRC bridge is gone.

**Moderation**
- Ranks are enforced: a moderator acts only on players, an admin on players and moderators, and nobody acts on themselves. Java let a moderator act on an admin who wasn't also flagged mod.
- Banning someone offline replies `<name> banned`. A ban also revokes the player's REST tokens and ends their game right away as a loss. Guests can only be kicked.
- `sudo set password` hides the password in the echo and the logs, and revokes the player's tokens.
- `sudo list` output is sorted, without a trailing comma, and lists each player once.
- `mod`, `admin` and `bot` say when the player already has, or doesn't have, that role.
- New: `sudo revoke <name>` (admin only).

**Accounts**
- Java accepted some non-ASCII passwords through a legacy fallback. That fallback is gone, so those players need to reset their password.
