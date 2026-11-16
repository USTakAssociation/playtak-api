#!/usr/bin/env bash
# Switches back from the Go tak-server to the Java server. Safe to run again.
#
# Usage, as root:
#   bash rollback.sh beta|prod                 keep the databases as they are
#   bash rollback.sh beta|prod --restore-db    also restore the newest backup
#
# The Java server runs fine on migrated databases, so restoring is rarely
# needed. Restoring loses every game and account change since the backup;
# the databases it replaces are kept beside them as *.before-restore.
set -euo pipefail

env=${1:-}
case "$env" in
beta | prod) ;;
*)
	echo "usage: rollback.sh beta|prod [--restore-db]" >&2
	exit 2
	;;
esac
restore=${2:-}
go_unit=tak-server-$env.service
java_unit=takserver-$env.service
backups=/var/backups/tak-server-$env

echo "stopping $go_unit"
systemctl disable --now "$go_unit" 2>/dev/null || true

if [ "$restore" = "--restore-db" ]; then
	latest=$(ls -1d "$backups"/*/ 2>/dev/null | sort | tail -n1)
	[ -n "$latest" ] && [ -f "$latest/paths" ] || { echo "no backup found in $backups" >&2; exit 1; }
	{ read -r players_db && read -r games_db; } <"$latest/paths"
	echo "restoring databases from $latest"
	cp -f "$players_db" "$players_db.before-restore"
	cp -f "$games_db" "$games_db.before-restore"
	cp -f "$latest/players.db" "$players_db"
	cp -f "$latest/games.db" "$games_db"
elif [ -n "$restore" ]; then
	echo "unknown option $restore" >&2
	exit 2
fi

echo "starting $java_unit"
systemctl enable --now "$java_unit"
systemctl is-active --quiet "$java_unit" && echo "rolled back: $java_unit is running"
