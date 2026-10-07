#!/usr/bin/env bash
# Create `player_games` view: each row is a (game, distinct player) pair.
# (A game appears once as its white player and once as its black player;
# using an additional condition so that self games only appear once.)
# -- a game appears once as its white player and once as its black, or
#
# Result: while using the same shape as the `games` table, we can search for
# a player on either side (solving the mirror=true case). Currently, this
# has good performance characteristics.
#
# Recommended that run profile script after migration:
#   2026-08-snapshot-player-games-view-query-performance.sh

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
