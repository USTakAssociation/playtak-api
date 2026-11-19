#!/usr/bin/env bash
# Add the auth_tokens table used by the Go server's REST auth endpoints.
# Tokens are stored as SHA-256 hashes; the raw token is never written.
#
# Requires 2026-09-players-id-primary-key.sh to have run first, since the
# foreign key needs players.id to be a primary key.
#
# Idempotent (safe to re-run).
set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
playersdb="${1:-$scriptpath/../../playtakdb/players.db}"
[ -f "$playersdb" ] || { echo "players.db not found at $playersdb" >&2; exit 1; }

pk=$(sqlite3 "$playersdb" "SELECT pk FROM pragma_table_info('players') WHERE name = 'id';")
if [ "$pk" != "1" ]; then
	echo "players.id is not a primary key; run 2026-09-players-id-primary-key.sh first" >&2
	exit 1
fi

sqlite3 "$playersdb" <<'SQL'
.bail on
BEGIN IMMEDIATE;
CREATE TABLE IF NOT EXISTS auth_tokens (
	token_hash BLOB PRIMARY KEY,
	player_id  INTEGER NOT NULL REFERENCES players (id) ON DELETE CASCADE,
	kind       TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires ON auth_tokens (expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_player ON auth_tokens (player_id);
COMMIT;
SQL
echo "auth_tokens is present"
