#!/usr/bin/env bash
# Runs every migration, in order. The one list of migrations: deploy.sh and
# create_databases.sh both call this. Each migration is idempotent, so this is
# safe to run again.
#
# Usage: run-all.sh <players.db> <games.db>
set -eo pipefail

[ $# -eq 2 ] || { echo "usage: run-all.sh <players.db> <games.db>" >&2; exit 2; }
players_db=$1
games_db=$2
dir=$(cd "$(dirname "$0")" && pwd)

# Add new migrations at the end, with the database they change.
bash "$dir/2026-05-add-increment-scales.sh" "$games_db"
bash "$dir/2026-08-add-nocase-player-indexes.sh" "$games_db"
bash "$dir/2026-09-players-id-primary-key.sh" "$players_db"
bash "$dir/2026-09-add-auth-tokens.sh" "$players_db"
