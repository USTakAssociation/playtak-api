#!/usr/bin/env bash
# Snapshot of how the player_games view plans the games-history player search.
# Run before/after 2026-08-add-player-games-view.sh (and after any SQLite upgrade).
#
# Expect: MERGE (UNION ALL) with each arm a `SEARCH games USING INDEX
# idx_games_player_{white,black} (player_{white,black}=? AND rowid>?)`. If it
# regresses to a SCAN or a temp b-tree, the view stopped flattening and the
# service's player path needs to move back to explicit SQL.
#
# usage: 2026-08-snapshot-player-games-view-query-performance.sh [path/to/games.db]
set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"
[ -f "$gamesdb" ] || { echo "games.db not found at $gamesdb" >&2; exit 1; }

PRELAUNCH_ID=7989

if ! sqlite3 "$gamesdb" "SELECT 1 FROM sqlite_master WHERE type='view' AND name='player_games';" | grep -q 1; then
	echo "player_games view not present -- run 2026-08-add-player-games-view.sh first" >&2
	exit 1
fi

plan() { # $1 label, $2 sql
	echo "  $1:"
	sqlite3 "$gamesdb" "EXPLAIN QUERY PLAN $2" | sed 's/^/    /'
}

echo "db: $gamesdb"
echo
plan "page (one player, id sort, floor)" \
	"SELECT * FROM player_games WHERE player_name = 'someplayer' COLLATE NOCASE AND id > $PRELAUNCH_ID ORDER BY id DESC LIMIT 50;"
plan "page, deep (OFFSET 2000)" \
	"SELECT * FROM player_games WHERE player_name = 'someplayer' COLLATE NOCASE AND id > $PRELAUNCH_ID ORDER BY id DESC LIMIT 50 OFFSET 2000;"
plan "count" \
	"SELECT COUNT(*) FROM player_games WHERE player_name = 'someplayer' COLLATE NOCASE AND id > $PRELAUNCH_ID;"
