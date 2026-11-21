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
# Steps: stop the servers (Java, and the NestJS API, which tak-server
# replaces), back up both databases, run the migrations, install the binary,
# its systemd unit and the hourly rating timer, start it, and check it
# answers. If any step after stopping the servers fails, rollback.sh starts
# Java and NestJS again. A failure after the backup but before the Go server starts
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
rate_unit=tak-server-rate-$env
# The NestJS API, run by pm2 as $run_user. Its rating job's cursor is in its
# working directory.
node_app=playtak-api-$env
node_cursor=/var/apps/playtak-api-$env/previous.txt
backups=/var/backups/tak-server-$env
# The server runs as the user that owns the databases, so the WAL files
# SQLite creates beside them (-wal, -shm) stay writable by everything else
# that opens them.
run_user=ubuntu

for f in tak-server .env rollback.sh migrations; do
	[ -e "$release/$f" ] || { echo "missing $f in $release" >&2; exit 1; }
done
command -v sqlite3 >/dev/null || { echo "sqlite3 is not installed" >&2; exit 1; }
id -u "$run_user" >/dev/null 2>&1 || { echo "user $run_user does not exist" >&2; exit 1; }

# Read one setting from .env. Not sourced, so secrets are never run as shell.
setting() { { grep -E "^$1=" "$release/.env" || true; } | tail -n1 | cut -d= -f2- | sed -E "s/^[\"'](.*)[\"']$/\\1/"; }
players_db=$(setting TAK_PLAYERS_DB)
games_db=$(setting TAK_GAMES_DB)
telnet_port=$(setting TAK_TELNET_ADDR)
telnet_port=${telnet_port##*:}
cursor=$(setting TAK_RATING_CURSOR_FILE)
cursor=${cursor:-$(dirname "$games_db")/previous.txt}
for v in players_db games_db telnet_port; do
	[ -n "${!v}" ] || { echo "$v is not set in .env" >&2; exit 1; }
done
for db in "$players_db" "$games_db"; do
	[ -f "$db" ] || { echo "database not found: $db" >&2; exit 1; }
done

# What a failure rolls back to depends on how far the deploy got:
#   stopped:  the databases are untouched; start Java.
#   backed-up: restore the backup taken after stopping (lossless), start Java.
#   started:  set before Go starts, not after the health check: once running,
#             Go may write (registrations, tokens, games) even if it then
#             fails the check, and a restore would lose that. Keep the data
#             and start Java, which runs on the migrated schema.
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

pm2() { sudo -u "$run_user" -i pm2 "$@"; }

echo "stopping servers"
systemctl stop "$go_unit" 2>/dev/null || true
systemctl stop "$java_unit" 2>/dev/null || true
# Stopped, not deleted, so rollback.sh can start it again. Saved so a reboot
# doesn't bring it back; its rating job and Go's would rate games twice.
if pm2 describe "$node_app" >/dev/null 2>&1; then
	pm2 stop "$node_app" >/dev/null || true # checked below
	pm2 save >/dev/null || true
fi
# A server that won't stop still holds the databases: leave everything as it
# is rather than back up or migrate underneath it.
for unit in "$go_unit" "$java_unit"; do
	if systemctl is-active --quiet "$unit"; then
		echo "$unit did not stop; deploy aborted, nothing changed" >&2
		exit 1
	fi
done
if pm2 pid "$node_app" 2>/dev/null | grep -q '[1-9]'; then
	echo "$node_app did not stop; deploy aborted, nothing changed" >&2
	exit 1
fi

# Backed up only after stopping, so no write can fall between the backup and
# the migrations; restoring it then loses nothing.
trap on_failure ERR

stamp=$(date +%Y%m%d-%H%M%S)
echo "backing up databases to $backups/$stamp"
# Owner-only: the backups hold password hashes and emails. chmod as well as
# mkdir -m, so a directory made by an older deploy is locked down too.
mkdir -p "$backups/$stamp"
chmod 700 "$backups" "$backups/$stamp"
sqlite3 "$players_db" ".backup '$backups/$stamp/players.db'"
sqlite3 "$games_db" ".backup '$backups/$stamp/games.db'"
chmod 600 "$backups/$stamp/players.db" "$backups/$stamp/games.db"
# The rating cursor that matches these databases, for rollback.sh --restore-db.
for f in "$cursor" "$node_cursor"; do
	if [ -f "$f" ]; then
		cp "$f" "$backups/$stamp/previous.txt"
		break
	fi
done
# Written last: rollback.sh only restores backups that have it.
printf '%s\n%s\n' "$players_db" "$games_db" >"$backups/$stamp/paths"
phase=backed-up

echo "running migrations"
bash "$release/migrations/run-all.sh" "$players_db" "$games_db"
# The backup and migrations ran as root: if SQLite left WAL files behind,
# hand them back, or the server couldn't write to the database.
for db in "$players_db" "$games_db"; do
	chown "$run_user:" "$db"
	chown -f "$run_user:" "$db-wal" "$db-shm" "$db-journal" 2>/dev/null || true # usually absent
done
# Written by an earlier deploy that ran the server as root.
pending=$(setting TAK_PENDING_RESULTS_FILE)
chown -f "$run_user:" "${pending:-$(dirname "$games_db")/pending-game-results.jsonl}" 2>/dev/null || true

echo "installing to $app_dir"
mkdir -p "$app_dir"
install -m 0755 "$release/tak-server" "$app_dir/tak-server.new"
mv -f "$app_dir/tak-server.new" "$app_dir/tak-server"
install -m 0600 -o "$run_user" "$release/.env" "$app_dir/.env"
install -m 0755 "$release/rollback.sh" "$app_dir/rollback.sh"

# The rating job carries on from where the NestJS job stopped. Once Go has
# its own cursor, that one is kept: rollback.sh moves it back to NestJS.
if [ ! -f "$cursor" ]; then
	if [ ! -f "$node_cursor" ]; then
		echo "no rating cursor: neither $cursor nor $node_cursor exists" >&2
		on_failure
	fi
	install -m 0644 -o "$run_user" -g "$(id -gn "$run_user")" "$node_cursor" "$cursor"
fi

# TODO: the server runs as $run_user with no sandboxing, so a remote code
# execution bug gets that user's files. Once the NodeJS API and the Java
# server are retired, run tak-server in a rootless podman container instead
# (or at least a dedicated user plus NoNewPrivileges, ProtectSystem=strict,
# ReadWritePaths= for the DB directory and PrivateTmp).
cat >"/etc/systemd/system/$go_unit" <<EOF
[Unit]
Description=PlayTak Go server ($env)
After=network.target

[Service]
WorkingDirectory=$app_dir
ExecStart=$app_dir/tak-server
User=$run_user
Type=simple
Restart=on-failure
RestartSec=10
# Lets the server close connections and save pending results before exit.
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
EOF

# The rating job, hourly at minute 19 as the NestJS job ran. A oneshot
# service never runs twice at once, and Persistent= makes up a run missed
# while the machine was down. Logs: journalctl -u $rate_unit
cat >"/etc/systemd/system/$rate_unit.service" <<EOF
[Unit]
Description=PlayTak rating job ($env)

[Service]
Type=oneshot
WorkingDirectory=$app_dir
ExecStart=$app_dir/tak-server rate
User=$run_user
EOF
cat >"/etc/systemd/system/$rate_unit.timer" <<EOF
[Unit]
Description=PlayTak rating job ($env), hourly

[Timer]
OnCalendar=*-*-* *:19:00
Persistent=true

[Install]
WantedBy=timers.target
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
systemctl enable --now "$rate_unit.timer"

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
