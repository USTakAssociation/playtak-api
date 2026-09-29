#!/usr/bin/env bash
# Rebuild the games player indexes as COLLATE NOCASE so the games-history
# player search (`player_white LIKE ?` / `player_black LIKE ?`, no wildcard)
# can use an index instead of scanning ~860k rows. Also drops the mistyped
# idx_gamess_id (`id` is INTEGER PRIMARY KEY, already the rowid).

# STRONGLY RECOMMENDED: evaluate effect by running before/after this change:
#   scripts/db-profiling/2026-08-snapshot-games-table-player-query-performance.sh
#
# Idempotent (safe to re-run).
set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"
[ -f "$gamesdb" ] || { echo "games.db not found at $gamesdb" >&2; exit 1; }

sqlite3 "$gamesdb" <<'SQL'
.bail on
-- DDL is transactional: readers see the old BINARY index until COMMIT, then the
-- new NOCASE one, with no unindexed window. `.bail on` + the open transaction
-- means a failed CREATE rolls back instead of reaching COMMIT. BEGIN IMMEDIATE
-- takes the write lock up front so a busy db fails cleanly, not mid-rebuild.
BEGIN IMMEDIATE;
DROP INDEX IF EXISTS idx_games_player_white;
DROP INDEX IF EXISTS idx_games_player_black;
CREATE INDEX idx_games_player_white ON games (player_white COLLATE NOCASE);
CREATE INDEX idx_games_player_black ON games (player_black COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_games_date ON games (date);
DROP INDEX IF EXISTS idx_gamess_id;
COMMIT;
-- Run an ANALYZE; it likely hasn't been run in prod (sqlite_stat1 absent in anon db).
-- Running it can fix improper query planning.
ANALYZE games;
SQL

echo "Rebuilt games player indexes as NOCASE on $gamesdb."
