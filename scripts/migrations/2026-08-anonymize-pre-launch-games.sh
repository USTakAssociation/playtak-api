#!/usr/bin/env bash
# Set both player columns to "Anon" on playtak's pre-2016-04-23 games
# (date <= 1461430800000, ~7,983 rows) -- matches what the anon export does.
# Lets a follow-up drop `date > 1461430800000` from generateSearchQuery.
# Swap PLACEHOLDER for a per-name map if the early match structure matters.
# Idempotent.
set -eo pipefail

db="${1:-$(cd "$(dirname "$0")" && pwd)/../../playtakdb/games.db}"
CUTOFF=1461430800000
PLACEHOLDER='Anon'
[ -f "$db" ] || { echo "games.db not found at $db" >&2; exit 1; }

rows=$(sqlite3 "$db" "SELECT COUNT(*) FROM games WHERE date <= $CUTOFF;")
sqlite3 -bail "$db" "BEGIN IMMEDIATE;
UPDATE games SET player_white = '$PLACEHOLDER', player_black = '$PLACEHOLDER' WHERE date <= $CUTOFF;
COMMIT;"
leaked=$(sqlite3 "$db" "SELECT COUNT(*) FROM games WHERE date <= $CUTOFF AND (player_white <> '$PLACEHOLDER' OR player_black <> '$PLACEHOLDER');")
echo "scrubbed $rows rows; $leaked not scrubbed (expect 0)"
