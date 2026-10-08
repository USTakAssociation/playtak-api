#!/usr/bin/env bash
# Snapshot of what the games-history player-search queries plan to, plus the
# games index list and whether ANALYZE has run. Run it against a games.db before
# and after 2026-08-add-nocase-player-indexes.sh (or any index change) and diff
# the output.
#
# Expect, after that migration: the LIKE page query goes SCAN -> SEARCH USING
# INDEX, and sqlite_stat1 becomes populated.
#
# usage: 2026-08-snapshot-games-table-player-query-performance.sh [path/to/games.db]
set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"
[ -f "$gamesdb" ] || { echo "games.db not found at $gamesdb" >&2; exit 1; }

FLOOR=1461430800000
PRELAUNCH_ID=7989

plan() { # $1 = label, $2 = sql
	echo "  $1:"
	sqlite3 "$gamesdb" "EXPLAIN QUERY PLAN $2" | sed 's/^/    /'
}

echo "db: $gamesdb"
echo
echo "indexes on games:"
sqlite3 "$gamesdb" "SELECT '  ' || name || '  ' || COALESCE(sql, '(auto)')
                    FROM sqlite_master WHERE type = 'index' AND tbl_name = 'games'
                    ORDER BY name;"
echo "sqlite_stat1 rows: $(sqlite3 "$gamesdb" "SELECT COUNT(*) FROM sqlite_stat1;" 2>/dev/null || echo 'table absent')"
echo
echo "query plans:"
plan "LIKE page (pre-migration shape)" \
	"SELECT * FROM games WHERE player_white LIKE 'someplayer' AND date > $FLOOR ORDER BY id DESC LIMIT 50;"
plan "LIKE mirror count (pre-migration shape)" \
	"SELECT COUNT(*) FROM games WHERE (player_white LIKE 'someplayer' AND date > $FLOOR) OR (player_black LIKE 'someplayer' AND date > $FLOOR);"
plan "= COLLATE NOCASE page (current getAll shape)" \
	"SELECT * FROM games WHERE player_white = 'someplayer' COLLATE NOCASE AND id > $PRELAUNCH_ID ORDER BY id DESC LIMIT 50;"
plan "UNION-arm count (current getAll fast-path shape)" \
	"SELECT COUNT(*) FROM (SELECT id FROM games WHERE player_white = 'someplayer' COLLATE NOCASE AND id > $PRELAUNCH_ID
	 UNION SELECT id FROM games WHERE player_black = 'someplayer' COLLATE NOCASE AND id > $PRELAUNCH_ID);"
