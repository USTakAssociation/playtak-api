#!/usr/bin/env bash
# Create the `player_games` view: games flattened so each row is one (game,
# distinct player) pair -- a game appears once as its white player and once as its
# black, or just once if white == black. The games-history player search reads
# this instead of `(player_white = ? OR player_black = ?)`; on the view a plain
# `player_name = ? AND id > ? ORDER BY id DESC LIMIT n` plans as MERGE (UNION ALL)
# over the two NOCASE indexes (no OR, no temp b-tree). See games.service.ts and
# the profiling script.
#
# A view is a stored query -- no storage, no triggers, nothing to keep in sync.
# `games.*` so it needs no column list and picks up future games columns.
# `player_black <> player_white` on the second arm keeps a self-game to one row.
#
# Needs the NOCASE player indexes (2026-08-add-nocase-player-indexes.sh) to be
# fast. Idempotent.
set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"
[ -f "$gamesdb" ] || { echo "games.db not found at $gamesdb" >&2; exit 1; }

sqlite3 "$gamesdb" <<'SQL'
.bail on
DROP VIEW IF EXISTS player_games;
CREATE VIEW player_games AS
	SELECT games.*, player_white AS player_name FROM games
	UNION ALL
	SELECT games.*, player_black AS player_name FROM games WHERE player_black <> player_white;
SQL

echo "Created player_games view on $gamesdb."
