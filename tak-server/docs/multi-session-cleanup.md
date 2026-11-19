# Multi-session cleanup

The server runs single-session by default (`TAK_ALLOW_MULTIPLE_SESSIONS=false`):
a player has one connection, and a new login replaces it. This matches the
current playtak-ui. Multi-session comes with the JSON protocol, with one game
per session.

Code that exists only to keep single-session behaviour is marked with a
`// single-session:` comment. Once multi-session is fully deployed and the
setting is removed, delete each marked branch and the setting itself, so
legacy rules don't build up in the code.

Find them all with:

```sh
grep -rn "single-session:" internal cmd
```

## Marked checks

| Where | What it does in single-session mode |
| --- | --- |
| `internal/session/manager.go` `bind` | A new login sends "You've logged in from another window" to the old connection and disconnects it. |
| `internal/game/manager.go` `Accept` | Starting a game stops the players' observing, because the client shows a single board. |
| `internal/game/manager.go` `addObserver` | A player in a game can't observe another game. |

Also remove `AllowMultipleSessions` from `session.Config`, `game.Config`,
`config.Config`, `.env.example` and `cmd/server/main.go`.

## Still player-level, to change for one game per session

These aren't single-session checks, but they assume a player's sessions all
share one game. In multi-session mode today, every one of a player's
sessions gets their game's events. These need to move to per-session seats
with the JSON protocol:

- Game seats are identified by player name, and `Game.send` sends to the
  player, which means all of their sessions (`internal/game/game.go`).
- Every login replays the player's game in progress to the new session
  (`Session.resumeGame` in `internal/session/account.go`).
- A disconnect is reported to the game only once the player has no
  connections left (`Session.Closed`).

Keep this file up to date when adding a `// single-session:` marker.
