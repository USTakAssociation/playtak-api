# Phase 2: Games history, ratings, events and the rating job in Go

Phase 2 moves what the NestJS API (`api/`) does into tak-server, so the
NestJS service can be shut down. This covers the games history, ratings and
events endpoints, the PTN export, and the hourly rating job. Branch:
`api-migration`, cut from `go-migration`.

## Decisions

| Question | Decision |
| --- | --- |
| Where does the rating job run? | `tak-server rate`, a subcommand of the same binary, run by cron. It isn't a goroutine inside the game server, so it uses no memory between runs, a panic in it can't take down live games, and changing it doesn't need a server restart. It shares `internal/db` and the config. |
| Must ratings match Node? | Yes, exactly. See [Rating parity](#rating-parity). |
| Player name search | Strict `= ? COLLATE NOCASE`. `LIKE` and `%` wildcards are gone (a `%` is now a literal character). |
| Pre-2016 games | No longer hidden or shown as "Anon", in search or in PTN. If that becomes a problem, edit the DB directly. The rating job still ignores them, as before. |
| PR #62 (player_games view) | Not merged. The Go query does the same job without a view or migration; see [Games history queries](#games-history-queries). |
| Events source | The sheet published to the web as CSV (`TAK_EVENTS_CSV_URL`). No Google SDK or service account. Cached 30 minutes; if Google fails, the last copy is served. |
| Endpoint paths | Exactly the NestJS ones (`/v1/games-history`, `/v1/ratings`, `/events`, `/health`), on the existing HTTP listener, so clients don't change. At cutover, api.playtak.com is pointed at that listener. |
| Not rating a player | `players.unrated = 1`, as with Node. Their games are recorded as unrated for both sides, their rating shows as 0 at game start, and they leave the ratings list. Nothing sets it yet except SQL. |

## Layout

Same split as seeks: HTTP in `internal/api`, SQL in `internal/db/*`, logic
in its own package.

| What | Where |
| --- | --- |
| Routes, query parsing, JSON shapes | `internal/api/history.go` (games history, ratings, `/health`, CORS) |
| PTN export | `internal/api/ptn.go`. It formats a games row and never touches the rules engine, so it sits beside its only caller. |
| Events feed | `internal/api/events.go` |
| Games history SQL | `internal/db/games/history.go` |
| Ratings SQL | `internal/db/players/ratings.go` |
| Rating math and job | `internal/rating` (`rating.go` is the math, `job.go` the database and files) |
| `rate` subcommand | `cmd/server/main.go` |

New settings: `TAK_EVENTS_CSV_URL`, `TAK_PUBLIC_GAMES_DB`,
`TAK_RATING_CURSOR_FILE`, `TAK_RATING_LIST_FILE` (see the README).

## Rating job

`rating.Run` reads the cursor, all players and up to 50,000 games after the
cursor, rates them in memory, then writes:

1. Every games row and player row, in **one transaction across both
   databases**. players.db is attached to the games.db connection, and
   `BEGIN IMMEDIATE` takes both write locks up front. Node committed
   games.db and players.db separately.
2. After the commit, the cursor file (`"<id> hash"`, Node's format, so either
   job can take over from the other).
3. The ratings page list (`ratinglist.json.gz`).

Both files are replaced atomically (written to a temp file, then renamed).
The run takes about 0.6 s per 50,000 games; an hourly run is well under a
second. While it commits, the game server's writes wait on SQLite's busy
timeout; inserts retry for up to 15 s.

### Rating parity

Ported line for line from `ratings.service.ts`, including the details that
decide the numbers:

- Games go in ID order, at most 50,000 a run, and only those after the
  named-accounts cutoff (2016-04-23).
- The cursor stops at the first eligible game that might still be in
  progress: 6 or fewer characters of notation, no result, and started in
  the last 6 hours. The next run picks it up from there.
- Any other game the job can't rate is written with rating changes of -2000.
- Players are matched by exact, case-sensitive name, first match wins. 346
  names have a duplicate differing only in case.
- Unrated players count as absent, and their rating is treated as 0.
- `Math.round` is reproduced (`jsRound`): Go's `math.Round` rounds -2.5 to
  -3 where JavaScript gives -2.
- The ratings list is sorted, stably, on the unrounded participation rating.

Checked three ways:

| Check | Result |
| --- | --- |
| `TestMatchesNodeJob` (CI): a seeded 414-game fixture covering every rule above, rated in two runs, compared with Node's output for the same fixture (`internal/rating/testdata/golden.json`) | Identical |
| Full history replay: all ratings reset, all 856,783 local games rated from scratch by both jobs | Every game row identical; every player identical (largest float difference 2e-14); the ratings list has the same entries, with 32 rows swapped among players whose displayed numbers are identical |
| Incremental: one run on top of the existing ratings, as in prod | Identical, including the ratings list |

Node took 89 s for the full history and Go 12 s.

The Node comparison tooling (the Node runner, golden generator, replay and
compare scripts) is in `tests/rating-parity/`, which is gitignored. It needs
the NestJS API and goes away with it. The golden file and fixture stay in
`internal/rating/testdata`, so the CI check doesn't need Node.

### Where Go deliberately differs from Node

These only show up on errors Node mishandled. None change results on good
data.

| Node | Go |
| --- | --- |
| An unreadable `fatigue` value throws (`parseFatigue` is passed unbound, so `this.logger` is undefined). The run rolls back, but because the error doesn't set `has_error`, it still overwrites `ratinglist.json.gz` with an **empty list**. No current rows trigger it. | Treated as no fatigue. |
| games.db commits before players.db, so a players failure leaves the two out of step. | One transaction for both. |
| A missing cursor file throws. | Also an error. Starting from 0 would rate every game twice. |
| Fatigue is written JSON-encoded twice (`"{\"85\":0.2}"`). | Written as plain JSON. Both versions read both forms. |
| `participation_rating` uses `Date.now()` per player. | One timestamp per run. The difference is milliseconds out of a 240-day half-life. |

Known gap (marked `shortcut:` in `job.go`): if the process dies between the
commit and writing the cursor, the next run rates those games again. Node has
the same window. Closing it means keeping the cursor in the database, which
can wait until the Node job is retired.

## Games history queries

Every filter from the NestJS API is kept: `id` (`5`, `5-10`, `5,7,9`), `date`
(`x`, `a-b`, `>x`, `<x`), `player_white`, `player_black`, `game_result` (a
result, or `X-0` / `0-X`), `size`, `timertime`, `timerinc`,
`extra_time_amount`, `extra_time_trigger`, `increment_scales`, `type`
(`normal`, `tournament`, `unrated`), `mirror`, `sort`, `order`, `limit`,
`page`, `skip`. `komi` was documented but never applied in Node; it is now.

**Why no OR.** Through the `modernc` driver with bound parameters, SQLite
3.53 won't plan `player_white = ? OR player_black = ?` across the two NOCASE
indexes, so every mirrored search scanned the whole table (0.7-0.85 s). The
sqlite3 CLI hid this, because it plans with the literal values. A mirrored
search is therefore two queries, one per color:

- Page: `first UNION ALL second-excluding-first ORDER BY ... LIMIT`. With the
  default id sort, SQLite merges both indexes already in order.
- Count: `count(*)` over `SELECT id ... UNION SELECT id ...`, which reads only
  the indexes and removes self-games by id.

Timings through the Go server on a copy of the local games.db (856k games):

| Search | Before (OR) | Now |
| --- | --- | --- |
| AaaarghBot, either color (162k games), page 1 or 40 | 0.85 s | 0.07 s |
| Rare player (cutak_bot) or missing name, either color | 0.75 s | < 0.01 s |
| One color only | — | < 0.01 s |

Still slow, about the cost of a full scan, as in Node: naming both players
(0.4 s), result plus size filters on the biggest bot (1 s), and sorting a big
bot's games by date (0.7 s). A `(player_white, player_black)` index would fix
the first if it matters.

## API changes clients can see

| Change | Why |
| --- | --- |
| Rate limits: 60/min per IP for games history and ratings, 30/min for events | Node's limits were configured as `ttl: 60`, which @nestjs/throttler v6 reads as milliseconds, so there was effectively no limit. |
| Games and ratings pages are capped at 1000 rows; `id`/`date` lists at 100 values | `limit` was unbounded, every games row includes the full move list, and each listed value is a SQL parameter. |
| Unknown `sort`, bad `order`, unparseable filters return 400 | Node put `sort` and `type` straight into SQL. |
| Missing game: 404 (was 200 with an empty body). Missing player rating: 404 (was 500). Events failure: 502 (was 200 with an error object). | |
| Errors are `{"error": "<code>"}` | The format the rest of tak-server uses (Node used `{statusCode, message, error}`). |
| `X-0` / `0-X` match R, F and 1 wins only | Node's `LIKE '%-0'` also matched `0-0`, which marks games in progress or aborted. |
| `/v1/games-history/db` returns `size`, `mtime` and `mtimeMs` only | The games history site only reads `size` and `mtime`. |
| CORS is `*` on the public endpoints | The data is public and read-only; no config needed. |
| No Swagger UI, no helmet headers | Not ported. |

## Cutover

Prod settings, mapped from the NestJS `.env`:

| NestJS | tak-server |
| --- | --- |
| `DB_PATH = /var/apps/playtak-server-prod/` | `TAK_GAMES_DB=/var/apps/playtak-server-prod/games.db`, `TAK_PLAYERS_DB=/var/apps/playtak-server-prod/players.db` |
| `ANON_DB_PATH = /var/www/html/games_anon.db.gz` | `TAK_PUBLIC_GAMES_DB=/var/www/html/games_anon.db.gz` |
| `RATING_OUTPUT_PATH = /var/www/html/` | `TAK_RATING_LIST_FILE=/var/www/html/ratinglist.json.gz` |
| `PREVIOUS_FILE = previous.txt` (relative to the Node process's working directory) | `TAK_RATING_CURSOR_FILE`, default `/var/apps/playtak-server-prod/previous.txt` |
| `RATING_CRON_VALUE = 0 19 * * * *` (minute 19 of every hour) | systemd timer `tak-server-rate-prod.timer`, installed by `deploy.sh` |
| `CORS_DOMAIN = playtak.com, ptn.ninja, attak.club, kati.exoticorn.de` | `*` on the public endpoints, which covers all four |
| `PORT`, `NODE_ENV`, `APP_ENV` | not needed |

`deploy.sh` does the cutover; see the README. It stops the NestJS API
(and with it the Node rating job), copies Node's `previous.txt` to the Go
cursor, and installs the hourly `tak-server-rate-<env>.timer`, run as
`ubuntu`, who owns `/var/www/html/` (the ratings list is written to a temp
file there and renamed into place). Once Go answers, it points nginx's
`upstream playtak-api-<env>` at the Go HTTP port. `rollback.sh` reverses it
all: it stops the timer, moves the cursor back to Node, starts NestJS and
points nginx back. Node can read everything Go writes.

Still by hand: once Go has run without a rollback for a while, `pm2 delete`
the NestJS apps, and delete `api/` and `tests/rating-parity/`.

TODO: the public anonymized games DB (`games_anon.db.gz`) is still built by
`/anon.sh` from ubuntu's crontab, outside this repo. It is only a snapshot
for the community to download; search and PTN no longer anonymize. The
script copies `games.db` with `cp`, which in WAL mode can miss recent games
or catch a half-written checkpoint. Fix: copy with `sqlite3 .backup`, set
the copy to `journal_mode=DELETE`, and run it from a timer in `deploy.sh`.

## Open questions

- Should opponents still gain or lose rating from games against a player
  marked `unrated`? Today, like Node, neither side is rated.
- Should `/v1/ratings/{name}` show 0 for unrated players? It currently
  returns their stored rating, as Node did.
- Do we want a mod or admin command to set `unrated`?
