#!/usr/bin/env bash
# Adds the increment_scales column to the games table for existing databases.
# Safe to re-run: ALTER is skipped if the column already exists.
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
