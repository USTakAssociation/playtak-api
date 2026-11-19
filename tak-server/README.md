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
- [Repository layout](#repository-layout)
- [Packages](#packages)
- [Running locally](#running-locally)
- [Configuration](#configuration)
- [Testing](#testing)
- [HTTP API](#http-api)
- [Rate limits](#rate-limits)
- [Database migrations](#database-migrations)
- [Deploying](#deploying)
- [Changes from the Java server](#changes-from-the-java-server)

## How it fits together

```
 telnet :10000 ──┐
                 ├─ transport ─ session ─┬─ auth ─────────── db/players
 websocket :9999 ┘   (lines)   (one per  ├─ seek
                               connection)├─ online
                                         ├─ game ── tak      db/games
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
- **online** holds the players who are logged in, and guests. Registered
  players are loaded from the database when they log in; nobody else is
  held in memory.
- **Finished games** are written to a pending-results file first, then
  saved to the games DB in the background, with retries. A busy database
  or a restart can't lose a result.

## Repository layout

```
tak-server/
├── cmd/server/     entry point (main.go)
├── internal/       all server code, one package per area (see Packages)
├── deploy/         deploy.sh and rollback.sh, run on the server by GitHub Actions
├── docs/           json-protocol.md (draft), multi-session-cleanup.md, go-migration-plan.md
├── .env.example    every setting, with defaults
└── README.md
```

Related code outside this folder:

| Path | What it is |
| --- | --- |
| `../scripts/migrations/` | Database migrations, run by `deploy.sh` |
| `../scripts/development/` | `create_databases.sh` and `add_user.sh` for local setup |
| `../tests/` | Compatibility suite, run against a live server |
| `../.github/workflows/tak-server-*.yml` | CI and deploys |

Inside a package, files are split by area. The two largest:

| Package | Files |
| --- | --- |
| `internal/game` | `game.go` (the game goroutine, moves, resign, timeouts), `actions.go` (undo, draw, give time), `observe.go`, `reconnect.go` (disconnects and rejoining), `clock.go`, `manager.go` (starting games, the game list, results) |
| `internal/session` | `session.go` (connection lifecycle and command dispatch), `account.go` (login, registration, passwords), `game.go`, `seek.go`, `chat.go`, `sudo.go`, `manager.go` (all connections, logins, presence) |

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
| `internal/online` | Who is online: the registry of logged-in players and guests. |
| `internal/seek` | Open seeks and rematches. |
| `internal/game` | Live games: the game goroutine, clocks, undo/draw/give time, observers, disconnects, and saving results. |
| `internal/tak` | The rules of Tak: placing, moving, roads, flat wins, TPS. |
| `internal/chat` | Shout, rooms and Tell, and keeping gagged players' messages to themselves. |
| `internal/moderation` | `sudo` commands for moderators and admins. |
| `internal/api` | The HTTP REST API. |
| `internal/clientip` | Finds the client's real IP behind a reverse proxy, trusting proxy headers only from configured proxies. |
| `internal/ratelimit` | Per-key token buckets (per IP, per username), built on `golang.org/x/time/rate`. |
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
| `TAK_SMTP_*` | empty | Outgoing email. Empty means emails are logged instead; their bodies only at `debug` level, since they can hold passwords. |
| `TAK_SERVICE_API_KEY` | empty | Shared secret for service-to-service calls such as `PUT /api/v1/seeks`. Empty disables them. |
| `TAK_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `TAK_TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | Reverse proxies whose `X-Real-IP` / `X-Forwarded-For` headers are believed. See [Rate limits](#rate-limits). |
| `TAK_LOGIN_LIMIT` | `username` | `username` counts failed logins per username as well as per IP; `ip` limits per IP only. |

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

## Rate limits

Logins, registrations and password resets are limited inside
`auth.Service`, so the same limits apply over REST, telnet and WebSocket:

| What | Limit |
| --- | --- |
| Failed logins per username (unless `TAK_LOGIN_LIMIT=ip`) | 10 per 15 minutes; a successful login resets it |
| Failed logins per IP | 20 per minute; successful logins don't count, so many players behind one IP can all log in. Logins still being checked count, so parallel attempts can't get past either login limit |
| Registrations per IP | 5 per hour |
| Password reset emails per username | 3 per hour |
| `POST /api/auth/refresh`, `/logout`, `/guest` per IP | 30 per minute, shared |
| `GET /api/v1/seeks` per IP | 60 per minute |

A limited REST request gets `429 Too Many Requests` with a `Retry-After`
header and `{"error":"rate_limited"}`. Over the text protocol, a limited
login gets the usual `Authentication failure`, and registration and reset
requests get a `Registration Error:` or `Reset Token Error:` saying to try
later, so existing clients need no changes. Every refusal is logged at warn
level. Counters are kept in memory and reset on restart.

"Per IP" means per IPv4 address, or per IPv6 /64, since one machine or home
network usually gets a whole /64. Each limit tracks up to 100,000 keys
(about 12 MB). If a flood of addresses or names fills one, keys it already
tracks stay limited, but new ones are let through uncounted until old entries
expire (a minute for failed logins), rather than locking out every new user.
The hourly cleanup logs a warning with how many went uncounted.

**Client IPs behind nginx.** The server only sees the address that
connected to it, which behind nginx is nginx itself. nginx must pass the
real address in `X-Real-IP` (see [Deploying](#deploying)), and the server
believes that header only on connections from `TAK_TRUSTED_PROXIES`. A
client connecting directly, for example to port 9999, can't fake its IP by
sending the header. Telnet doesn't go through nginx, so it always sees real
addresses. Without the header, everyone behind nginx shares one per-IP
limit; the per-username limit still protects accounts.

## Database migrations

Schema changes are dated shell scripts in `../scripts/migrations`. They're
safe to run more than once.

- **Deploys run them automatically.** `deploy/deploy.sh` backs up both
  databases and runs every migration before starting the server, so there's
  nothing to run by hand. See [Deploying](#deploying).
- **Local development:** `create_databases.sh` creates fresh databases and
  runs every migration.
- **Adding a migration:** add the script, then add it to the end of
  `../scripts/migrations/run-all.sh`, the one list both `deploy.sh` and
  `create_databases.sh` run.

At startup the server checks that the players database has had the
migrations it relies on (the `id` primary key and the `auth_tokens` table)
and refuses to start if not, naming the script to run. That only matters if
the server is started without `deploy.sh`.

## Deploying

Production runs the binary under systemd on the VPS, next to the Java
server it replaces. GitHub Actions builds and deploys it:

| Workflow | Runs on | Does |
| --- | --- | --- |
| `tak-server-ci.yml` | pull requests to `dev` or `main` | `go vet`, `go test -race`, checks the deploy scripts |
| `tak-server-cd-dev.yml` | push to `dev` (or run by hand) | tests, builds, deploys to beta |
| `tak-server-cd-prod.yml` | push to `main` (or run by hand) | tests, builds, deploys to prod |

The deploy job copies a release (binary, `.env`, migrations and the scripts
in `deploy/`) to the server and runs `deploy/deploy.sh beta|prod` as root.

### What deploy.sh does

Safe to run again. On each run it:

1. Stops the Go and Java servers for that environment. If either is still
   running afterwards, it stops there and changes nothing.
2. Backs up both databases to `/var/backups/tak-server-<env>/<time>/`. The
   backup is taken after stopping, so nothing can be written between the
   backup and the migrations.
3. Runs every migration. Each is safe to run more than once.
4. Installs the binary and `.env` to `/var/apps/tak-server-<env>/` and the
   unit `tak-server-<env>.service`.
5. Disables the Java unit (`takserver-<env>.service`), so a reboot can't
   start both, and checks it really is disabled. Then it starts the Go unit.
6. Checks the server sends `Welcome!` on its telnet port within 30 seconds.
7. Deletes old backups, keeping the 10 newest. This runs only after a
   successful deploy, so failed deploys can't push out the last good backup.

If any step after stopping fails, it rolls back to Java and fails the job:

| Failed during | Rollback |
| --- | --- |
| the backup | Start Java. The databases were not touched. |
| migrations or install | Restore the backup, then start Java. Nothing ran since the backup, so no data is lost, and a half-applied migration is undone. |
| starting or the health check | Start Java and keep the data. Go may have saved games, and Java runs on the migrated databases. |

If the rollback itself fails, neither server is running. The job log then
shows `ROLLBACK FAILED` with the command to retry it by hand (the release
directory is kept after a failure), and the script exits with code 2
instead of 1.

Games in progress when the server stops end without a result, as with the
Java server. Warn players first with `sudo broadcast Message ...`.

### Rolling back

On the server, as root:

```sh
bash /var/apps/tak-server-prod/rollback.sh prod              # back to Java, keep the data
bash /var/apps/tak-server-prod/rollback.sh prod --restore-db # also restore the newest backup
```

The Java server runs on the migrated databases, so restoring is rarely
needed. Restoring loses everything since the backup; the replaced files are
kept as `*.before-restore`. `rollback.sh` refuses to restore while either
server is still running, and removes any leftover journal or WAL files so
they can't be replayed onto the restored databases.

### Settings: the TAK_ENV secret

Each GitHub environment (`Beta`, `Prod`) needs a `TAK_ENV` secret holding
the whole `.env` file. It replaces the Java `PROPERTIES` secret. Set the
ports and database paths explicitly, because beta and prod share the
machine. Beta, matching its Java config:

```sh
TAK_TELNET_ADDR=:10002
TAK_WS_ADDR=:10001
TAK_HTTP_ADDR=:10003
TAK_PLAYERS_DB=/var/apps/playtak-server-beta/players.db
TAK_GAMES_DB=/var/apps/playtak-server-beta/games.db
TAK_RECONNECT_TIMEOUT=4m
TAK_SMTP_HOST=mail.lunanode.net
TAK_SMTP_USER=admin@playtak.com
TAK_SMTP_PASSWORD=...
TAK_SMTP_FROM=admin@playtak.com
```

Prod uses ports `10000`, `9999` and `9998` and its own database paths.
See [`.env.example`](.env.example) for every setting.

### Before the first deploy

- `sqlite3` must be installed on the server; the migrations and backups use it.
- nginx: add `proxy_set_header X-Real-IP $remote_addr;` to every location
  that proxies to the server: `/ws` on playtak.com and beta.playtak.com,
  and `/api/` on beta.playtak.com. nginx connects from 127.0.0.1, which the
  default `TAK_TRUSTED_PROXIES` trusts.
- Once the Go server has replaced Java for good, delete the Java workflows
  (`server-cd-dev.yml`, `server-cd-prod.yml`). A push under `server/` would
  otherwise start the Java server on the same ports.
- Announce the changes listed below, including asking anyone who can't log
  in to reset their password.

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
- No profanity filter: chat isn't masked and usernames aren't checked for bad words. Moderators handle abuse with gag, kick and ban. `sudo reload wordconfig` now replies `Command not found`.

**Moderation**
- Ranks are enforced: a moderator acts only on players, an admin on players and moderators, and nobody acts on themselves. Java let a moderator act on an admin who wasn't also flagged mod.
- Banning someone offline replies `<name> banned`. A ban also revokes the player's REST tokens and ends their game right away as a loss. Guests can only be kicked.
- `sudo set password` hides the password in the echo and the logs, and revokes the player's tokens.
- `sudo list` output is sorted, without a trailing comma, and lists each player once.
- `mod`, `admin` and `bot` say when the player already has, or doesn't have, that role.
- New: `sudo revoke <name>` (admin only).

**Connections**
- WebSocket connections end with a close code that says why. Java just closed the socket.

  | Code | When |
  | --- | --- |
  | 1000 normal | the client quit |
  | 1001 going away | nothing received for 60 seconds (Java also dropped these, without a code), or the server is shutting down |
  | 1008 policy violation | logged in from another window, kicked or banned |
  | 1011 internal error | a line over 64 KB, or a client too slow to keep up |

- A client that falls 1024 lines behind is disconnected, so one slow connection can't hold up the server. Java kept queuing. Telnet clients are disconnected the same way, without a code.

**Accounts**
- Java accepted some non-ASCII passwords through a legacy fallback. That fallback is gone, so those players need to reset their password.
- Logins, registrations and password resets are rate limited (see [Rate limits](#rate-limits)). Java had no limits.
