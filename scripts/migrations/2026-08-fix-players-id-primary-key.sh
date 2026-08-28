#!/usr/bin/env bash
# Rebuilds the players table so `id` is a real INTEGER PRIMARY KEY (rowid alias).
#
# Databases created by the old create_databases.sh got `id INT PRIMARY_KEY`, which
# SQLite parses as a column whose type name is "INT PRIMARY_KEY" -- with NO primary
# key, uniqueness or NOT NULL constraint (fixed for fresh dbs in
# b553140548fa309104f96dc13d89005cd0644965). This migration repairs existing dbs.
#
# It performs the officially recommended table rebuild
# (https://www.sqlite.org/lang_altertable.html#otheralter) inside a transaction:
# create new table, copy every column by name, drop old, rename, replay indexes
# and triggers. Only the `id` column declaration is changed.
#
# SAFETY: it refuses to touch the db (exit 1, nothing written) unless the
# conversion is provably lossless:
#   - players table exists and has an `id` column
#   - no NULL id values
#   - no duplicate id values
#   - every id value already has SQLite storage type 'integer'
# If `id` is already a proper INTEGER PRIMARY KEY it exits 0 without changes.
# Safe to re-run.
set -euo pipefail

scriptpath=$(dirname "$(readlink -f "$0")")
playersdb="${1:-$scriptpath/../../playtakdb/players.db}"

if [ ! -f "$playersdb" ]; then
	echo "players.db not found at $playersdb" >&2
	exit 1
fi

q() { sqlite3 "$playersdb" "$1"; }

# table_info rows: cid|name|type|notnull|dflt_value|pk
info=$(q "PRAGMA table_info(players);")
if [ -z "$info" ]; then
	echo "players table not found in $playersdb" >&2
	exit 1
fi

id_row=$(printf '%s\n' "$info" | awk -F'|' '$2=="id"')
if [ -z "$id_row" ]; then
	echo "players table has no 'id' column; refusing to guess. No changes made." >&2
	exit 1
fi
id_type=$(printf '%s' "$id_row" | awk -F'|' '{print toupper($3)}')
id_pk=$(printf '%s' "$id_row" | awk -F'|' '{print $6}')

if [ "$id_pk" -ge 1 ] 2>/dev/null && [ "$id_type" = "INTEGER" ]; then
	echo "players.id is already INTEGER PRIMARY KEY. Nothing to do."
	exit 0
fi

# ---- safety checks -----------------------------------------------------------
nulls=$(q "SELECT count(*) FROM players WHERE id IS NULL;")
dups=$(q "SELECT count(*) FROM (SELECT id FROM players GROUP BY id HAVING count(*) > 1);")
nonint=$(q "SELECT count(*) FROM players WHERE typeof(id) <> 'integer';")

fail=0
[ "$nulls" = 0 ]  || { echo "ABORT: $nulls row(s) have a NULL id." >&2; fail=1; }
[ "$dups" = 0 ]   || { echo "ABORT: $dups id value(s) are duplicated." >&2; fail=1; }
[ "$nonint" = 0 ] || { echo "ABORT: $nonint row(s) have a non-integer id." >&2; fail=1; }
if [ "$fail" -ne 0 ]; then
	echo "players.id cannot be made INTEGER PRIMARY KEY safely. No changes made." >&2
	exit 1
fi

# ---- reconstruct column list, changing only `id` ---------------------------
cols_def=""
cols_names=""
while IFS='|' read -r _cid name ctype notnull dflt _pk; do
	[ -n "$name" ] || continue
	if [ -n "$cols_def" ]; then
		cols_def="$cols_def, "
		cols_names="$cols_names, "
	fi
	cols_names="$cols_names\"$name\""
	if [ "$name" = "id" ]; then
		cols_def="$cols_def\"id\" INTEGER PRIMARY KEY"
		continue
	fi
	decl="\"$name\" $ctype"
	[ "$notnull" = "1" ] && decl="$decl NOT NULL"
	[ -n "$dflt" ] && decl="$decl DEFAULT $dflt"
	cols_def="$cols_def$decl"
done <<EOF
$info
EOF

# ---- capture indexes / triggers to replay after the rebuild ---------------
aux=$(q "SELECT sql FROM sqlite_master WHERE tbl_name='players' AND type IN ('index','trigger') AND sql NOT NULL;")
aux_sql=""
if [ -n "$aux" ]; then
	while IFS= read -r stmt; do
		[ -n "$stmt" ] && aux_sql="$aux_sql
$stmt;"
	done <<EOF
$aux
EOF
fi

before=$(q "SELECT count(*) FROM players;")

sqlite3 "$playersdb" <<SQL
PRAGMA foreign_keys=OFF;
BEGIN;
CREATE TABLE players__new ($cols_def);
INSERT INTO players__new ($cols_names) SELECT $cols_names FROM players;
DROP TABLE players;
ALTER TABLE players__new RENAME TO players;
$aux_sql
COMMIT;
PRAGMA foreign_keys=ON;
SQL

after=$(q "SELECT count(*) FROM players;")
newpk=$(q "PRAGMA table_info(players);" | awk -F'|' '$2=="id"{print $6}')

if [ "$before" != "$after" ] || [ "$newpk" != "1" ]; then
	echo "ERROR: rebuild verification failed (rows $before -> $after, id pk=$newpk)." >&2
	exit 1
fi

echo "players.id is now INTEGER PRIMARY KEY ($after rows preserved)."
