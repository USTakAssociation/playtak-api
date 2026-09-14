#!/usr/bin/env bash
# Adds the clocks column to the games table for existing databases:
#   - clocks (per-ply remaining time in ms, aligned with notation; see
#     server/src/main/java/tak/ClockHistory.java)
# Safe to re-run: the ALTER is skipped if the column already exists.
set -e

scriptpath=$(dirname "$(readlink -f "$0")")
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"

if [ ! -f "$gamesdb" ]; then
	echo "games.db not found at $gamesdb" >&2
	exit 1
fi

if sqlite3 "$gamesdb" "PRAGMA table_info(games);" | awk -F'|' '{print $2}' | grep -qx clocks; then
	echo "clocks column already present, nothing to do."
else
	sqlite3 "$gamesdb" "ALTER TABLE games ADD COLUMN clocks TEXT;"
	echo "Added clocks column to $gamesdb."
fi
