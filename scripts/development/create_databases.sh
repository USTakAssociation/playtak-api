#!/bin/bash

# set the playtakdb folder to the parent directory variable
# (cd/pwd, not `readlink -f`, which stock macOS/BSD readlink does not support)
scriptpath=$(cd "$(dirname "$0")" && pwd)
dbpath="$scriptpath/../../playtakdb"

# set the database paths
playersdb="$dbpath/players.db"
gamesdb="$dbpath/games.db"

# check if playtakdb folder exists
if [ ! -d $dbpath ]; then
	mkdir "$dbpath"
fi

# remove existing
if [ -f $playersdb ]; then
	rm "$playersdb"
fi
if [ -f $gamesdb ]; then
	rm "$gamesdb"
fi

# create db, tables

# CAVEAT 1 (players): `id INT PRIMARY_KEY` matches prod but is actually is a
# bug. `PRIMARY_KEY` (with underscore) is not a keyword, so `id` is a plain
# nullable INT column, not primary key and not rowid alias. Kept as-is to match
# prod. See CAVEAT 2 for how this doesn't affect production checks for the PK.

# CAVEAT 2 (players) : missing prod player indexes Confirmed that there are
# additional prod indexes: - idx_players_id (to compensate for CAVEAT 1) -
# idx_players_name Left alone in development DB creation, for now.

echo "CREATE TABLE players (id INT PRIMARY_KEY, name VARCHAR(20), password VARCHAR(50), email VARCHAR(50), rating real default 1000, boost real default 750, ratedgames int default 0, maxrating real default 1000, ratingage real default 0, ratingbase int default 0, unrated int default 0, isbot int default 0, fatigue text default '{}', is_admin int default 0, is_mod int default 0, is_gagged int default 0, is_banned int default 0, participation_rating int default 1000);" | sqlite3 $playersdb
echo "CREATE TABLE games (id INTEGER PRIMARY KEY, date INT, size INT, player_white VARCHAR(20), player_black VARCHAR(20), notation TEXT, result VARCAR(10), timertime INT DEFAULT 0, timerinc INT DEFAULT 0, rating_white int default 1000, rating_black int default 1000, unrated int default 0, tournament int default 0, komi int default 0, pieces int default -1, capstones int default -1, rating_change_white int default 0, rating_change_black int default 0, extra_time_amount int default 0, extra_time_trigger int default 0, increment_scales int default 0, opening VARCHAR(20) default 'swap');" | sqlite3 $gamesdb

# Apply the dated (idempotent) migrations so a fresh dev setup matches migrated
# prod. (Add new ones here with needed params; should be idempotent.)

migrations="$scriptpath/../migrations"
bash "$migrations/2026-05-add-increment-scales.sh" "$gamesdb"
bash "$migrations/2026-08-add-nocase-player-indexes.sh" "$gamesdb"
bash "$migrations/2026-08-add-player-games-view.sh" "$gamesdb"
