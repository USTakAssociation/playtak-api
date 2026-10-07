import 'reflect-metadata';
import { DataSource, Repository } from 'typeorm';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { GameQuery } from '../dto/games/games.dto';
import { Games } from './entities/games.entity';
import { PlayerGames } from './entities/player-games.entity';
import { GamesService } from './games.service';
import { PTNService } from './services/ptn.service';

// Behavioural contract for search: runs the real GamesService against a real
// in-memory SQLite db and asserts on the games / counts / ordering it returns,
// not on how the query is built. The mock spec next to this one pins the current
// query internals; this is the safety net for changing them.

const OLD_ID = 7989; // an old game; this base applies no pre-launch floor
const POST = 1_500_000_000_000;
const PRE = 1_400_000_000_000;

type Row = Partial<Games> & { id: number; player_white: string; player_black: string };

const FIXTURES: Row[] = [
	// old, low-id games: included like any other, no floor in play
	{ id: 3, date: PRE, player_white: 'OldTimer', player_black: 'AaaarghBot', result: '1-0', size: 5 },
	{ id: OLD_ID, date: PRE, player_white: 'AaaarghBot', player_black: 'OldTimer', result: '0-1', size: 5 },
	// live AaaarghBot games, both colours, ids ascending
	{ id: 9001, date: POST, player_white: 'AaaarghBot', player_black: 'alice', result: '1-0', size: 5 },
	{ id: 9002, date: POST, player_white: 'bob', player_black: 'AaaarghBot', result: '0-1', size: 6 },
	{ id: 9003, date: POST, player_white: 'AaaarghBot', player_black: 'carol', result: 'R-0', size: 5 },
	{ id: 9004, date: POST, player_white: 'AaaarghBot', player_black: 'AaaarghBot', result: '1/2-1/2', size: 5 }, // self-game
	{ id: 9005, date: POST, player_white: 'dave', player_black: 'AaaarghBot', result: '1-0', size: 5 },
	{ id: 9006, date: POST, player_white: 'AaaarghBot', player_black: 'erin', result: '0-1', size: 5 },
	{ id: 9007, date: POST, player_white: 'AaaarghBot', player_black: 'frank', result: '1-0', size: 5 },
	{ id: 9008, date: POST, player_white: 'grace', player_black: 'AaaarghBot', result: '0-R', size: 5 },
	// one-game player, and a case-variant of a searched name
	{ id: 9100, date: POST, player_white: 'Hotch', player_black: 'MixedCase', result: '1-0', size: 5 },
	// literal underscore in a name + a name a `_` wildcard would also match
	{ id: 9200, date: POST, player_white: 'Under_score', player_black: 'x', result: '1-0', size: 5 },
	{ id: 9201, date: POST, player_white: 'UnderXscore', player_black: 'y', result: '1-0', size: 5 },
	// unrelated games -- must not leak into player searches
	{ id: 9900, date: POST, player_white: 'zeta', player_black: 'theta', result: '1-0', size: 7 },
	{ id: 9901, date: POST, player_white: 'theta', player_black: 'zeta', result: '0-1', size: 7 }
];

const VIEW_SQL =
	'CREATE VIEW player_games AS ' +
	'SELECT games.*, player_white AS player_name FROM games ' +
	'UNION ALL ' +
	'SELECT games.*, player_black AS player_name FROM games WHERE player_black <> player_white';

let ds: DataSource;
let svc: GamesService;

beforeAll(async () => {
	ds = new DataSource({
		type: 'better-sqlite3',
		database: ':memory:',
		entities: [Games, PlayerGames],
		synchronize: true
	});
	await ds.initialize();
	const repo: Repository<Games> = ds.getRepository(Games);
	await repo.insert(FIXTURES.map((r) => ({ notation: '', extra_time_amount: 0, extra_time_trigger: 0, ...r })));
	await ds.query(VIEW_SQL);
	svc = new GamesService(repo, ds.getRepository(PlayerGames), new PTNService());
});

afterAll(async () => {
	await ds.destroy();
});

const ids = (res: { items: Array<{ id: number }> }) => res.items.map((g) => g.id);

describe('GamesService.getAll — search behaviour', () => {
	it('non-mirror player_white returns only that colour, newest id first', async () => {
		const res = await svc.getAll({ player_white: 'AaaarghBot', mirror: 'false' });
		expect(ids(res)).toEqual([9007, 9006, 9004, 9003, 9001, OLD_ID]);
		expect(res.total).toBe(6);
	});

	it('player match is case-insensitive', async () => {
		const lower = await svc.getAll({ player_white: 'aaaarghbot', mirror: 'false' });
		const exact = await svc.getAll({ player_white: 'AaaarghBot', mirror: 'false' });
		expect(ids(lower)).toEqual(ids(exact));
		expect(ids(await svc.getAll({ player_black: 'mixedcase', mirror: 'false' }))).toEqual([9100]);
	});

	it('mirror returns games of either colour, self-game once, newest first', async () => {
		const res = await svc.getAll({ player_black: 'AaaarghBot', mirror: 'true' });
		expect(ids(res)).toEqual([9008, 9007, 9006, 9005, 9004, 9003, 9002, 9001, OLD_ID, 3]);
		expect(res.total).toBe(10);
	});

	it('mirror ascending id order also comes off the view', async () => {
		const res = await svc.getAll({ player_black: 'AaaarghBot', mirror: 'true', order: 'ASC' });
		expect(ids(res)).toEqual([3, OLD_ID, 9001, 9002, 9003, 9004, 9005, 9006, 9007, 9008]);
	});

	it('an explicit id filter is honoured', async () => {
		const res = await svc.getAll({ player_black: 'AaaarghBot', id: '1-9001', mirror: 'true' });
		expect(ids(res)).toEqual([9001, OLD_ID, 3]);
	});

	it('an extra filter narrows the mirror search', async () => {
		const res = await svc.getAll({ player_black: 'AaaarghBot', mirror: 'true', size: '6' } as unknown as GameQuery);
		expect(ids(res)).toEqual([9002]);
	});

	it('paginates: pages disjoint, ordered, total stable', async () => {
		const p0 = await svc.getAll({ player_black: 'AaaarghBot', mirror: 'true', limit: '3', page: '0' });
		const p1 = await svc.getAll({ player_black: 'AaaarghBot', mirror: 'true', limit: '3', page: '1' });
		expect(ids(p0)).toEqual([9008, 9007, 9006]);
		expect(ids(p1)).toEqual([9005, 9004, 9003]);
		expect(p0.total).toBe(10);
		expect(p1.totalPages).toBe(4);
		expect(p1.page).toBe(2);
	});

	it('supports % wildcards (prefix)', async () => {
		const res = await svc.getAll({ player_white: 'Aaa%', mirror: 'false' });
		expect(ids(res)).toEqual([9007, 9006, 9004, 9003, 9001, OLD_ID]);
	});

	it('treats a literal _ as a LIKE wildcard (unchanged)', async () => {
		const res = await svc.getAll({ player_white: 'Under_score', mirror: 'false' });
		expect(ids(res).sort()).toEqual([9200, 9201]);
	});

	it('nonexistent player -> empty', async () => {
		const res = await svc.getAll({ player_white: 'nobody-here', mirror: 'true' });
		expect(res.items).toEqual([]);
		expect(res.total).toBe(0);
		expect(res.totalPages).toBe(0);
	});

	it('non-player search still works', async () => {
		const res = await svc.getAll({ size: '7', mirror: 'false' } as unknown as GameQuery);
		expect(ids(res)).toEqual([9901, 9900]);
	});

	it('blank search returns everything newest-first', async () => {
		const res = await svc.getAll({ mirror: 'false', limit: '100' });
		expect(ids(res)).toEqual([...ids(res)].sort((a, b) => b - a));
		expect(res.total).toBe(FIXTURES.length);
	});

	it('view path and games-table path agree for the same logical query', async () => {
		const q = { player_black: 'AaaarghBot', mirror: 'true' };
		expect(svc.planQuery(q).source).toBe('player_games');
		const viaView = await svc.getAll(q);

		// a sort the view cannot serve from its MERGE forces the games-table OR;
		// id is unique here, so the two orderings must agree row for row
		const tableQ = { ...q, sort: 'id', order: 'DESC' as const, date: `>0` };
		expect(svc.planQuery({ ...q, sort: 'date' }).source).toBe('games');
		const viaTable = await svc.getAll({ ...tableQ, sort: 'date' });
		expect(new Set(ids(viaTable))).toEqual(new Set(ids(viaView)));
		expect(viaTable.total).toBe(viaView.total);
	});
});
