#!/usr/bin/env bash
# Make players.id a real primary key. The original schema declares
# `id INT PRIMARY_KEY`, and PRIMARY_KEY is not a keyword, so id has been a
# plain nullable column with no uniqueness guarantee. The Go server relies on
# id being INTEGER PRIMARY KEY so SQLite assigns new ids and auth_tokens can
# reference players(id).
#
# SQLite can't alter a column into a primary key, so the table is rebuilt from
# its own current definition (only the id column is changed). This keeps any
# extra columns a given database has, e.g. prod's r4..r8. Indexes are
# recreated, except idx_players_id, which the rowid makes redundant.
#
# Aborts without changes if any id is NULL or duplicated. Takes a write lock
# for the duration of the copy, so run it with the server stopped.
#
# Idempotent (safe to re-run).
set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
playersdb="${1:-$scriptpath/../../playtakdb/players.db}"
[ -f "$playersdb" ] || { echo "players.db not found at $playersdb" >&2; exit 1; }

pk=$(sqlite3 "$playersdb" "SELECT pk FROM pragma_table_info('players') WHERE name = 'id';")
if [ "$pk" = "1" ]; then
	echo "players.id is already the primary key; nothing to do"
	exit 0
fi

bad=$(sqlite3 "$playersdb" "SELECT (SELECT count(*) FROM players WHERE id IS NULL) + (SELECT count(*) FROM (SELECT id FROM players GROUP BY id HAVING count(*) > 1));")
if [ "$bad" != "0" ]; then
	echo "players has NULL or duplicate ids; fix them before migrating:" >&2
	sqlite3 "$playersdb" "SELECT 'NULL id: ' || rowid || ' ' || name FROM players WHERE id IS NULL; SELECT 'duplicate id: ' || id || ' x' || count(*) FROM players GROUP BY id HAVING count(*) > 1;" >&2
	exit 1
fi

others=$(sqlite3 "$playersdb" "SELECT count(*) FROM sqlite_master WHERE tbl_name = 'players' AND type IN ('trigger', 'view');")
if [ "$others" != "0" ]; then
	echo "players has triggers or views; this script doesn't know how to carry them over" >&2
	exit 1
fi

create=$(sqlite3 "$playersdb" "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'players';")
newcreate=$(printf '%s' "$create" | sed -E 's/^CREATE TABLE players/CREATE TABLE players_new/; s/\(id INT PRIMARY_KEY,/(id INTEGER PRIMARY KEY,/')
if [ "$newcreate" = "$(printf '%s' "$create" | sed -E 's/^CREATE TABLE players/CREATE TABLE players_new/')" ]; then
	echo "unexpected players definition; expected it to start with '(id INT PRIMARY_KEY,':" >&2
	echo "$create" >&2
	exit 1
fi

indexes=$(sqlite3 "$playersdb" "SELECT sql || ';' FROM sqlite_master WHERE type = 'index' AND tbl_name = 'players' AND sql IS NOT NULL AND name != 'idx_players_id';")
before=$(sqlite3 "$playersdb" "SELECT count(*) FROM players;")

sqlite3 "$playersdb" <<SQL
.bail on
BEGIN IMMEDIATE;
$newcreate;
INSERT INTO players_new SELECT * FROM players;
DROP TABLE players;
ALTER TABLE players_new RENAME TO players;
$indexes
COMMIT;
SQL

after=$(sqlite3 "$playersdb" "SELECT count(*) FROM players;")
echo "players.id is now INTEGER PRIMARY KEY ($after rows, was $before)"
