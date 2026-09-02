#!/usr/bin/env bash
# Adds the protocol 4 columns to the games table for existing databases:
#   - increment_scales (increment scaling)
#   - opening          (opening variant, e.g. Double Black Stack)
# Safe to re-run: each ALTER is skipped if its column already exists.
set -e

scriptpath=$(dirname "$(readlink -f "$0")")
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"

if [ ! -f "$gamesdb" ]; then
	echo "games.db not found at $gamesdb" >&2
	exit 1
fi

if sqlite3 "$gamesdb" "PRAGMA table_info(games);" | awk -F'|' '{print $2}' | grep -qx increment_scales; then
	echo "increment_scales column already present, nothing to do."
else
	sqlite3 "$gamesdb" "ALTER TABLE games ADD COLUMN increment_scales INTEGER DEFAULT 0;"
	echo "Added increment_scales column to $gamesdb."
fi

if sqlite3 "$gamesdb" "PRAGMA table_info(games);" | awk -F'|' '{print $2}' | grep -qx opening; then
	echo "opening column already present, nothing to do."
else
	sqlite3 "$gamesdb" "ALTER TABLE games ADD COLUMN opening VARCHAR(20) DEFAULT 'swap';"
	echo "Added opening column to $gamesdb."
fi
