#!/bin/bash
# set the playtakdb folder to the parent directory variable
$dbPath = "../../playtakdb"

#  check if playtakdb folder exists
if [ ! -d $dbpath ]; then
	mkdir $dbpath
fi
#remove existing
if [ -f $dbPath/players.db ]; then
	rm $dbPath/players.db
fi

if [ -f $dbPath/games.db ]; then
	rm $dbPath/games.db
fi

$playersdb = "$dbPath/players.db"
$gamesdb = "$dbPath/games.db"

#create db, tables
echo "CREATE TABLE players (id INT PRIMARY_KEY, name VARCHAR(20), password VARCHAR(50), email VARCHAR(50), rating real default 1000, boost real default 750, ratedgames int default 0, maxrating real default 1000, ratingage real default 0, ratingbase int default 0, unrated int default 0, isbot int default 0, fatigue text default '{}', is_admin int default 0, is_mod int default 0, is_gagged int default 0, is_banned int default 0, participation_rating int default 1000);" | sqlite3 $playersdb
echo "CREATE TABLE games (id INTEGER PRIMARY KEY, date INT, size INT, player_white VARCHAR(20), player_black VARCHAR(20), notation TEXT, result VARCAR(10), timertime INT DEFAULT 0, timerinc INT DEFAULT 0, rating_white int default 1000, rating_black int default 1000, unrated int default 0, tournament int default 0, komi int default 0, pieces int default -1, capstones int default -1, rating_change_white int default 0, rating_change_black int default 0, extra_time_amount int default 0, extra_time_trigger int default 0);" | sqlite3 $gamesdb
