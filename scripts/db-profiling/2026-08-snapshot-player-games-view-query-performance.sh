#!/usr/bin/env bash
# Snapshot of how the games-history mirror player search plans, on the `games`
# table (OR) and on the `player_games` view. Run before/after
# 2026-08-add-player-games-view.sh, and after any better-sqlite3 upgrade.
#
# Runs through better-sqlite3, NOT the sqlite3 CLI: they bundle different SQLite
# versions and pick different plans for this query. On 3.43 (the macOS CLI) the
# OR gets MULTI-INDEX OR + a temp b-tree; on 3.53 (better-sqlite3 13.x) it gets
# a plain SCAN. Only the engine the API links against tells you anything.
#
# Expect, for the view: MERGE (UNION ALL) over idx_games_player_{white,black}.
# That plan is already in id order, so a page is index-bounded whatever the
# player's game count. If it regresses to a SCAN or grows a temp b-tree, the
# view has stopped earning its keep.
#
# The OR row is the comparison, not a target: a SCAN there is fast only when the
# player is common enough that 50 matching rows turn up early.
#
# usage: 2026-08-snapshot-player-games-view-query-performance.sh [path/to/games.db] [player]

set -eo pipefail

scriptpath=$(cd "$(dirname "$0")" && pwd)
gamesdb="${1:-$scriptpath/../../playtakdb/games.db}"
player="${2:-someplayer}"
api="$scriptpath/../../api"
[ -f "$gamesdb" ] || { echo "games.db not found at $gamesdb" >&2; exit 1; }
gamesdb=$(cd "$(dirname "$gamesdb")" && pwd)/$(basename "$gamesdb")   # node runs from $api
[ -d "$api/node_modules/better-sqlite3" ] || { echo "run pnpm install in $api first" >&2; exit 1; }

cd "$api"
node -e '
const Database = require("better-sqlite3");
const [db, player] = process.argv.slice(1);
const d = new Database(db, { readonly: true });

if (!d.prepare("SELECT 1 FROM sqlite_master WHERE type=? AND name=?").get("view", "player_games")) {
	console.error("player_games view not present -- run 2026-08-add-player-games-view.sh first");
	process.exit(1);
}

const OR = "player_white = ? COLLATE NOCASE OR player_black = ? COLLATE NOCASE";
const VIEW = "player_name = ? COLLATE NOCASE";
const cases = [
	["games OR   page", `SELECT id FROM games WHERE ${OR} ORDER BY id DESC LIMIT 50`, [player, player]],
	["games OR   count", `SELECT COUNT(1) FROM games WHERE ${OR}`, [player, player]],
	["view       page", `SELECT id FROM player_games WHERE ${VIEW} ORDER BY id DESC LIMIT 50`, [player]],
	["view       page @2000", `SELECT id FROM player_games WHERE ${VIEW} ORDER BY id DESC LIMIT 50 OFFSET 2000`, [player]],
	["view       count", `SELECT COUNT(1) FROM player_games WHERE ${VIEW}`, [player]]
];

const median = (sql, params) => {
	d.prepare(sql).all(...params);
	const runs = [];
	for (let i = 0; i < 5; i++) {
		const t = process.hrtime.bigint();
		d.prepare(sql).all(...params);
		runs.push(Number(process.hrtime.bigint() - t) / 1e6);
	}
	return runs.sort((a, b) => a - b)[2];
};

console.log(`db:     ${db}`);
console.log(`player: ${player}`);
console.log(`sqlite: ${d.prepare("select sqlite_version() v").get().v} (via better-sqlite3)`);
console.log("");
for (const [label, sql, params] of cases) {
	const ms = median(sql, params).toFixed(1).padStart(7);
	const plan = d.prepare(`EXPLAIN QUERY PLAN ${sql}`).all(...params).map((r) => r.detail).join(" | ");
	console.log(`  ${label.padEnd(22)} ${ms}ms   ${plan}`);
}
' "$gamesdb" "$player"
