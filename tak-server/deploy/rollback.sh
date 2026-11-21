#!/usr/bin/env bash
# Switches back from the Go tak-server to the Java server and the NestJS API,
# and hands the rating job back to NestJS. Safe to run again.
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
rate_unit=tak-server-rate-$env
backups=/var/backups/tak-server-$env
app_dir=/var/apps/tak-server-$env
run_user=ubuntu
node_app=playtak-api-$env
node_cursor=/var/apps/playtak-api-$env/previous.txt

pm2() { sudo -u "$run_user" -i pm2 "$@"; }

# api.<env>.playtak.com reaches whichever API is live through one nginx
# upstream. point_api sets the port in that block only, checks the config
# and reloads; on a bad config it puts the file back. The copy is kept
# outside sites-enabled, where nginx would load it as a second config.
nginx_conf=/etc/nginx/sites-enabled/proxysettings
nginx_upstream=playtak-api-$env
node_api_port_file=$app_dir/nginx-api-port.node
api_port() { sed -n "/^upstream $nginx_upstream {/,/}/ s/.*127\.0\.0\.1:\([0-9]\+\).*/\1/p" "$nginx_conf"; }
point_api() {
	if [ -z "$(api_port)" ]; then
		echo "no 'upstream $nginx_upstream' block with a 127.0.0.1 port in $nginx_conf" >&2
		return 1
	fi
	cp "$nginx_conf" "$app_dir/proxysettings.before"
	sed -i --follow-symlinks "/^upstream $nginx_upstream {/,/}/ s/127\.0\.0\.1:[0-9]\+/127.0.0.1:$1/" "$nginx_conf"
	if ! nginx -t -q; then
		cp "$app_dir/proxysettings.before" "$(readlink -f "$nginx_conf")"
		echo "nginx rejected the config; left it as it was" >&2
		return 1
	fi
	systemctl reload nginx
}
# Read one setting from the installed .env, as deploy.sh does.
setting() { { grep -E "^$1=" "$app_dir/.env" 2>/dev/null || true; } | tail -n1 | cut -d= -f2- | sed -E "s/^[\"'](.*)[\"']$/\\1/"; }
cursor=$(setting TAK_RATING_CURSOR_FILE)
games_db=$(setting TAK_GAMES_DB)
if [ -z "$cursor" ] && [ -n "$games_db" ]; then
	cursor=$(dirname "$games_db")/previous.txt
fi

# Stop the Go rating job first, letting a run in progress finish: killed
# between its database commit and its cursor write, the cursor would be
# behind and NestJS would rate those games again.
systemctl disable --now "$rate_unit.timer" 2>/dev/null || true
for _ in $(seq 1 600); do
	systemctl is-active --quiet "$rate_unit.service" || break
	sleep 1
done
if systemctl is-active --quiet "$rate_unit.service"; then
	echo "$rate_unit.service is still running after 10 minutes; not continuing" >&2
	exit 1
fi

echo "stopping $go_unit"
systemctl disable --now "$go_unit" 2>/dev/null || true
if systemctl is-active --quiet "$go_unit"; then
	echo "$go_unit is still running; not continuing" >&2
	exit 1
fi

if [ "$restore" = "--restore-db" ]; then
	# The newest complete backup: deploy.sh writes "paths" last. Names are
	# timestamps, so the glob sorts oldest first.
	latest=
	for dir in "$backups"/*/; do
		if [ -f "${dir}paths" ]; then
			latest=$dir
		fi
	done
	[ -n "$latest" ] || { echo "no backup found in $backups" >&2; exit 1; }
	{ read -r players_db && read -r games_db; } <"$latest/paths"
	# Nothing may hold the files while they're replaced.
	if systemctl is-active --quiet "$java_unit"; then
		echo "$java_unit is running; stop it before restoring" >&2
		exit 1
	fi
	echo "restoring databases from $latest"
	# Through SQLite rather than cp: the databases are in WAL mode, so the
	# main file alone is missing recent writes, and copying over it or
	# deleting its -wal under another open connection (the NestJS API) can
	# corrupt it. .restore takes the write lock and other connections see the
	# restored data.
	for pair in "players.db=$players_db" "games.db=$games_db"; do
		db=${pair#*=}
		sqlite3 "$db" ".timeout 5000" ".backup '$db.before-restore'"
		chmod 600 "$db.before-restore" # password hashes and emails, like the backups
		sqlite3 "$db" ".timeout 5000" ".restore '$latest${pair%%=*}'"
		# Run as root: hand back any WAL files SQLite created.
		chown -f "$(stat -c %U:%G "$db")" "$db-wal" "$db-shm" 2>/dev/null || true
	done
elif [ -n "$restore" ]; then
	echo "unknown option $restore" >&2
	exit 2
fi

# Hand the rating cursor back to NestJS. After a restore, the cursor saved
# with that backup is the one that matches the databases. Otherwise Go's is
# the latest. It's moved, not copied, so the next deploy picks up NestJS's.
if [ -n "$restore" ] && [ -f "$latest/previous.txt" ]; then
	install -m 0644 -o "$run_user" -g "$(id -gn "$run_user")" "$latest/previous.txt" "$node_cursor"
	[ -z "$cursor" ] || rm -f "$cursor"
elif [ -n "$cursor" ] && [ -f "$cursor" ]; then
	mv -f "$cursor" "$node_cursor"
fi

echo "starting $java_unit"
systemctl enable --now "$java_unit"
if ! systemctl is-active --quiet "$java_unit"; then
	echo "$java_unit did not start; check: journalctl -u $java_unit" >&2
	exit 1
fi

if pm2 describe "$node_app" >/dev/null 2>&1; then
	echo "starting $node_app"
	pm2 start "$node_app" >/dev/null
	pm2 save >/dev/null
	if ! pm2 pid "$node_app" 2>/dev/null | grep -q '[1-9]'; then
		echo "$node_app did not start; check: sudo -u $run_user -i pm2 logs $node_app" >&2
		exit 1
	fi
fi
# Back to NestJS, if a deploy moved the API to Go.
if [ -f "$node_api_port_file" ]; then
	echo "pointing nginx upstream $nginx_upstream at port $(cat "$node_api_port_file")"
	point_api "$(cat "$node_api_port_file")"
fi
echo "rolled back: $java_unit and $node_app are running"
