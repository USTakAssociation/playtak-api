#!/usr/bin/env bash
# Delete playtak's pre-2016-04-23 games (date <= 1461430800000, ~7,983 rows).
# All "Anon" in the anon export -- no player identity, just a row count.
# Lets a follow-up drop `date > 1461430800000` from generateSearchQuery.
# Idempotent. No VACUUM (~1% of the file).
set -eo pipefail

db="${1:-$(cd "$(dirname "$0")" && pwd)/../../playtakdb/games.db}"
CUTOFF=1461430800000
[ -f "$db" ] || { echo "games.db not found at $db" >&2; exit 1; }

before=$(sqlite3 "$db" "SELECT COUNT(*) FROM games;")
sqlite3 -bail "$db" "BEGIN IMMEDIATE; DELETE FROM games WHERE date <= $CUTOFF; COMMIT;"
after=$(sqlite3 "$db" "SELECT COUNT(*) FROM games;")
echo "deleted $((before - after)) rows; $after remain"
