#!/usr/bin/env bash
# Installs the Go tak-server on this machine in place of the Java server, or
# upgrades an existing Go install. Safe to run again.
#
# Usage, from the unpacked release directory, as root:
#   bash deploy.sh beta|prod
#
# The release directory holds: tak-server (the binary), .env, deploy.sh,
# rollback.sh and migrations/. .env must set TAK_PLAYERS_DB, TAK_GAMES_DB and
# TAK_TELNET_ADDR explicitly (beta and prod share the machine, so relying on
# the default ports would collide).
#
# Steps: stop the servers, back up both databases, run the migrations,
# install the binary and its systemd unit, start it, and check it answers.
# If any step after stopping the servers fails, rollback.sh starts the Java
# server again. A failure after the backup but before the Go server starts
# also restores that backup: nothing has written since, so no data is lost,
# and a half-applied migration is undone.
set -Eeuo pipefail

env=${1:-}
case "$env" in
beta | prod) ;;
*)
	echo "usage: deploy.sh beta|prod" >&2
	exit 2
	;;
esac

release=$(cd "$(dirname "$0")" && pwd)
app_dir=/var/apps/tak-server-$env
go_unit=tak-server-$env.service
java_unit=takserver-$env.service
backups=/var/backups/tak-server-$env

for f in tak-server .env rollback.sh migrations; do
	[ -e "$release/$f" ] || { echo "missing $f in $release" >&2; exit 1; }
done
command -v sqlite3 >/dev/null || { echo "sqlite3 is not installed" >&2; exit 1; }

# Read one setting from .env. Not sourced, so secrets are never run as shell.
setting() { { grep -E "^$1=" "$release/.env" || true; } | tail -n1 | cut -d= -f2- | sed -E "s/^[\"'](.*)[\"']$/\\1/"; }
players_db=$(setting TAK_PLAYERS_DB)
games_db=$(setting TAK_GAMES_DB)
telnet_port=$(setting TAK_TELNET_ADDR)
telnet_port=${telnet_port##*:}
for v in players_db games_db telnet_port; do
	[ -n "${!v}" ] || { echo "$v is not set in .env" >&2; exit 1; }
done
for db in "$players_db" "$games_db"; do
	[ -f "$db" ] || { echo "database not found: $db" >&2; exit 1; }
done

# What a failure rolls back to depends on how far the deploy got:
#   stopped:  the databases are untouched; start Java.
#   backed-up: restore the backup taken after stopping (lossless), start Java.
#   started:  Go may have saved games; keep the data, start Java (it runs on
#             the migrated schema).
phase=stopped
on_failure() {
	trap - ERR
	echo "deploy failed at step '$phase'; rolling back to Java" >&2
	journalctl -u "$go_unit" -n 30 --no-pager >&2 2>/dev/null || true
	local restore=
	if [ "$phase" = backed-up ]; then
		restore=--restore-db
	fi
	if ! bash "$release/rollback.sh" "$env" ${restore:+"$restore"}; then
		cat >&2 <<-EOF
			ROLLBACK FAILED: neither server is running.
			Check: systemctl status $go_unit $java_unit
			Retry: bash $release/rollback.sh $env $restore
		EOF
		exit 2
	fi
	exit 1
}

echo "stopping servers"
systemctl stop "$go_unit" 2>/dev/null || true
systemctl stop "$java_unit" 2>/dev/null || true
# A server that won't stop still holds the databases: leave everything as it
# is rather than back up or migrate underneath it.
for unit in "$go_unit" "$java_unit"; do
	if systemctl is-active --quiet "$unit"; then
		echo "$unit did not stop; deploy aborted, nothing changed" >&2
		exit 1
	fi
done

# Backed up only after stopping, so no write can fall between the backup and
# the migrations; restoring it then loses nothing.
trap on_failure ERR

stamp=$(date +%Y%m%d-%H%M%S)
echo "backing up databases to $backups/$stamp"
mkdir -p "$backups/$stamp"
sqlite3 "$players_db" ".backup '$backups/$stamp/players.db'"
sqlite3 "$games_db" ".backup '$backups/$stamp/games.db'"
# Written last: rollback.sh only restores backups that have it.
printf '%s\n%s\n' "$players_db" "$games_db" >"$backups/$stamp/paths"
phase=backed-up

echo "running migrations"
# shortcut: the order is listed here and in scripts/development/create_databases.sh; keep both in sync when adding one.
m=$release/migrations
bash "$m/2026-05-add-increment-scales.sh" "$games_db"
bash "$m/2026-08-add-nocase-player-indexes.sh" "$games_db"
bash "$m/2026-09-players-id-primary-key.sh" "$players_db"
bash "$m/2026-09-add-auth-tokens.sh" "$players_db"

echo "installing to $app_dir"
mkdir -p "$app_dir"
install -m 0755 "$release/tak-server" "$app_dir/tak-server.new"
mv -f "$app_dir/tak-server.new" "$app_dir/tak-server"
install -m 0600 "$release/.env" "$app_dir/.env"
install -m 0755 "$release/rollback.sh" "$app_dir/rollback.sh"

cat >"/etc/systemd/system/$go_unit" <<EOF
[Unit]
Description=PlayTak Go server ($env)
After=network.target

[Service]
WorkingDirectory=$app_dir
ExecStart=$app_dir/tak-server
User=root
Type=simple
Restart=on-failure
RestartSec=10
# Lets the server close connections and save pending results before exit.
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
# Disabled so a reboot can't start both servers on the same ports.
systemctl disable "$java_unit" 2>/dev/null || true # fails if there's no Java unit
if systemctl is-enabled --quiet "$java_unit" 2>/dev/null; then
	echo "$java_unit is still enabled; a reboot would start both servers" >&2
	on_failure
fi
phase=started
systemctl enable --now "$go_unit"

echo "checking the server answers on port $telnet_port"
for _ in $(seq 1 30); do
	if line=$(timeout 2 bash -c "exec 3<>/dev/tcp/127.0.0.1/$telnet_port && head -n1 <&3" 2>/dev/null) &&
		[[ $line == Welcome!* ]]; then
		echo "deployed: $go_unit is running"
		# Deployed, so nothing below may roll back.
		trap - ERR
		# Keep the 10 newest backups, pruning only now so a run of failed
		# deploys can't push out the last good one. Names are timestamps, so
		# the glob sorts oldest first.
		backup_dirs=("$backups"/*/)
		if [ "${#backup_dirs[@]}" -gt 10 ]; then
			rm -rf "${backup_dirs[@]:0:${#backup_dirs[@]}-10}" || echo "warning: could not prune old backups in $backups" >&2
		fi
		exit 0
	fi
	sleep 1
done

echo "the Go server did not answer" >&2
on_failure
