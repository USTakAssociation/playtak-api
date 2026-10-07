import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { stat } from 'fs/promises';
import { Between, In, LessThan, Like, MoreThan, Raw, Repository } from 'typeorm';
import { LEGACY_GAMES_ANONYMIZED_FROM_RESULTS, LEGACY_GAMES_CUTOFF } from '../../config/feature-flags';
import { GameQuery } from '../dto/games/games.dto';
import { Games } from './entities/games.entity';
import { PlayerGames } from './entities/player-games.entity';
import { PTNService } from './services/ptn.service';

/**
 * Query helper: based on the param, chooses the right where clause.
 *
 * - If wildcard explicitly included, use LIKE (which in sqlite default to case-insensitive)
 * - Otherwise (default and most common case), use equality with COLLATE NOCASE, in order
 *   to use the case insensitive index performantly.
 */
const playerMatch = (value: string, param: string) => {
	if (/[%_]/.test(value)) return Like(`${value}`);
	return Raw((col) => `${col} = :${param} COLLATE NOCASE`, { [param]: value });
};

@Injectable()
export class GamesService {
	constructor(
		@InjectRepository(Games, 'games')
		private repository: Repository<Games>,
		@InjectRepository(PlayerGames, 'games')
		private playerGamesRepository: Repository<PlayerGames>,
		private ptnService: PTNService
	) {}

	validateIdQuery(id: string) {
		const regex = /^(?!.*,,)(?!.*--)\d+([-,\d]*\d+)?$/;
		if (!regex.test(id)) {
			return false;
		}

		// cannot contain both a hyphen and a comma
		if (id.includes('-') && id.includes(',')) {
			return false;
		}
		// if value has a hyphen check that the second number is greater than the first
		if (id.includes('-')) {
			const idArray = id.split('-');
			if (parseInt(idArray[0]) >= parseInt(idArray[1])) {
				return false;
			}
		}
		return true;
	}

	generateSearchQuery(query: GameQuery) {
		const search = {};
		if (query['id']) {
			search['id'] = parseInt(query['id']);
		}
		if (query['id'] && this.validateIdQuery(query['id'])) {
			if (query['id'].includes(',')) {
				const ids = query['id'].split(',');
				const arr = [];
				for (let i = 0; i < ids.length; i++) {
					arr.push(parseInt(ids[i]));
				}
				search['id'] = In(arr);
			}
			if (query['id'].includes('-')) {
				const ids = query['id'].split('-');
				search['id'] = Between(parseInt(ids[0]), parseInt(ids[1]));
			}
		}
		if (query['id'] && query['id'].includes('-')) {
			// remove duplicate hyphens
			query['id'] = query['id'].replace(/-{2,}/g, '-');
			const ids = query['id'].split('-');
			// make sure the first id is smaller than the second
			if (parseInt(ids[0]) > parseInt(ids[1])) {
				const temp = ids[0];
				ids[0] = ids[1];
				ids[1] = temp;
			}
			search['id'] = Between(parseInt(ids[0]), parseInt(ids[1]));
		}
		// date query handling for single value, between, greater than, less than
		if (query['date']) {
			if (query['date'].includes('-')) {
				// remove duplicate hyphens
				query['date'] = query['date'].replace(/-{2,}/g, '-');
				const dates = query['date'].split('-');
				// make sure the first date is smaller than the second
				if (parseInt(dates[0]) > parseInt(dates[1])) {
					const temp = dates[0];
					dates[0] = dates[1];
					dates[1] = temp;
				}
				search['date'] = Between(parseInt(dates[0]), parseInt(dates[1]));
			} else if (query['date'].startsWith('>')) {
				const dateValue = query['date'].substring(1);
				search['date'] = MoreThan(parseInt(dateValue));
			} else if (query['date'].startsWith('<')) {
				const dateValue = query['date'].substring(1);
				search['date'] = LessThan(parseInt(dateValue));
			} else {
				search['date'] = parseInt(query['date']);
			}
		}
		if (query['player_white']) {
			search['player_white'] = query['player_white'];
		}
		if (query['player_black']) {
			search['player_black'] = query['player_black'];
		}
		if (query['game_result']) {
			search['game_result'] = query['game_result'];
		}
		if (query['size']) {
			search['size'] = query['size'];
		}
		if (query['timertime']) {
			search['timertime'] = parseInt(query['timertime']);
		}
		if (query['timerinc']) {
			search['timerinc'] = parseInt(query['timerinc']);
		}
		if (query['extra_time_amount']) {
			search['extra_time_amount'] = parseInt(query['extra_time_amount']);
		}
		if (query['extra_time_trigger']) {
			search['extra_time_trigger'] = parseInt(query['extra_time_trigger']);
		}
		if (query['increment_scales']) {
			search['increment_scales'] = parseInt(query['increment_scales']);
		}
		if (query['type']) {
			search[query['type'].toLowerCase()] = 1;
		}
		const mirror = query.mirror === 'true' ? true : false;

		if (search['normal']) {
			search['tournament'] = 0;
			search['unrated'] = 0;
			delete search['normal'];
		}

		let player_search: boolean;
		const playerWhite = search['player_white'];
		const playerBlack = search['player_black'];
		if (playerWhite) {
			search['player_white'] = playerMatch(playerWhite, 'pw');
			player_search = true;
		}
		if (playerBlack) {
			search['player_black'] = playerMatch(playerBlack, 'pb');
			player_search = true;
		}

		if (search['game_result']) {
			if (search['game_result'] === 'X-0') {
				search['result'] = Like('%-0');
			} else if (search['game_result'] === '0-X') {
				search['result'] = Like('0-%');
			} else {
				search['result'] = search['game_result'];
			}
		}

		let mirrorSearch = {};
		if (mirror) {
			mirrorSearch = { ...search };
			delete mirrorSearch['player_black'];
			delete mirrorSearch['player_white'];
			if (playerWhite) {
				mirrorSearch['player_black'] = playerMatch(playerWhite, 'pwm');
				player_search = true;
			}
			if (playerBlack) {
				mirrorSearch['player_white'] = playerMatch(playerBlack, 'pbm');
				player_search = true;
			}
			if (search['game_result']) {
				switch (search['game_result']) {
					case 'X-0':
						mirrorSearch['result'] = Like('0-%');
						break;
					case '0-X':
						mirrorSearch['result'] = Like('%-0');
						break;
					case '1/2-1/2':
						mirrorSearch['result'] = '1/2-1/2';
						break;
					case '0-R':
						mirrorSearch['result'] = 'R-0';
						break;
					case 'R-0':
						mirrorSearch['result'] = '0-R';
						break;
					case 'F-0':
						mirrorSearch['result'] = '0-F';
						break;
					case '0-F':
						mirrorSearch['result'] = 'F-0';
						break;
					case '1-0':
						mirrorSearch['result'] = '0-1';
						break;
					case '0-1':
						mirrorSearch['result'] = '1-0';
						break;
					default:
						mirrorSearch['result'] = search['game_result'];
						break;
				}
			}
		}
		delete search['game_result'];
		delete mirrorSearch['game_result'];
		if (player_search && LEGACY_GAMES_ANONYMIZED_FROM_RESULTS) {
			search['date'] = MoreThan(LEGACY_GAMES_CUTOFF.toString());
			if (mirror) {
				mirrorSearch['date'] = MoreThan(LEGACY_GAMES_CUTOFF.toString());
			}
		}

		return { search, mirrorSearch, playerWhite, playerBlack };
	}

	/**
	 * A mirror search is "this player, on either side". Against the `games`
	 * table that is `player_white = ? OR player_black = ?`, and on the SQLite
	 * better-sqlite3 links against (3.53.4) that OR does not use the NOCASE
	 * player indexes at all -- it plans as `SCAN games`, walking the table in
	 * rowid order. That is quick only while matches are dense enough to fill a
	 * page early: for anyone but a mega-bot both the page and the count read
	 * all 870k rows (~60ms each, measured; see
	 * scripts/db-profiling/2026-08-snapshot-player-games-view-query-performance.sh).
	 *
	 * The `player_games` view (scripts/migrations/2026-08-add-player-games-view.sh)
	 * is the same OR hoisted into the schema, which SQLite can plan as MERGE
	 * (UNION ALL) over the two indexes -- already in id order, so a page is
	 * index-bounded whatever the player's game count.
	 *
	 * When the view can be used, planQuery returns the ordinary `search` object
	 * with the player predicate moved onto `player_name`; every other filter is
	 * side-independent and ANDs on without disturbing the MERGE.
	 */
	private canUsePlayerGamesView(query: GameQuery, playerWhite?: string, playerBlack?: string): boolean {
		// Without mirror, a player search already resolves to a single index.
		if (query.mirror !== 'true') return false;
		// Naming both players is "A vs B", not "X on either side": the view's one
		// player_name per row can only carry half of it. Naming neither has no
		// player predicate to hoist.
		const exactlyOnePlayer = !playerWhite !== !playerBlack;
		if (!exactlyOnePlayer) return false;
		// A mirrored game_result depends on which side matched (1-0 one way is 0-1
		// the other), so it cannot be a plain AND on the view.
		if (query.game_result) return false;
		// A wildcard makes the player predicate a LIKE range rather than a point
		// lookup, and the merged arms are then no longer id-ordered.
		if (/[%_]/.test(playerWhite || playerBlack)) return false;
		// Only an id sort comes out of the MERGE for free. Any other sort makes each
		// arm build its own temp b-tree, and measures no better than the table.
		return (query.sort || 'id') === 'id';
	}

	// Turn a query into { which table/view to read, the find `where` }.
	planQuery(query: GameQuery): { source: 'games' | 'player_games'; where: object | object[] } {
		const { search, mirrorSearch, playerWhite, playerBlack } = this.generateSearchQuery(query);

		if (this.canUsePlayerGamesView(query, playerWhite, playerBlack)) {
			// The player match Raw is column-agnostic, so it reads fine under
			// `player_name`; the rest of the filters carry over untouched.
			const { player_white, player_black, ...filters } = search as Record<string, unknown>;
			return { source: 'player_games', where: { ...filters, player_name: player_white ?? player_black } };
		}
		return { source: 'games', where: query.mirror === 'true' ? [search, mirrorSearch] : search };
	}

	async getAll(query?: GameQuery): Promise<any> {
		const limit = parseInt(query.limit) || 50;
		const page = parseInt(query.page) || 0;
		const skip = limit * page || parseInt(query.skip) || 0;
		const order: 'ASC' | 'DESC' = query.order || 'DESC';
		const sort = query.sort || 'id';
		const { source, where } = this.planQuery(query);
		try {
			const repo = source === 'player_games' ? this.playerGamesRepository : this.repository;
			const [items, total] = await repo.findAndCount({ where, order: { [sort]: order }, take: limit, skip });
			return {
				items: items || [],
				total: total || 0,
				page: page + 1,
				perPage: limit,
				totalPages: Math.ceil(total / limit)
			};
		} catch (error) {
			console.error(error);
			throw new Error('Could not get games. ' + error);
		}
	}

	async getOneByID(id: number): Promise<any> {
		try {
			const result = await this.repository.findOne({
				where: { id }
			});
			return result;
		} catch (error) {
			console.error(error);
			throw new Error('Could not get game by ID. ' + error);
		}
	}

	async getDBInfo() {
		try {
			const stats = await stat(process.env.ANON_DB_PATH);
			return {
				// Basic stats
				dev: stats.dev,
				mode: stats.mode,
				nlink: stats.nlink,
				uid: stats.uid,
				gid: stats.gid,
				rdev: stats.rdev,
				blksize: stats.blksize,
				ino: stats.ino,
				size: stats.size,
				blocks: stats.blocks,

				// Timestamp information (ensuring full timestamp data)
				atimeMs: stats.atimeMs,
				mtimeMs: stats.mtimeMs,
				ctimeMs: stats.ctimeMs,
				birthtimeMs: stats.birthtimeMs,

				// Adding formatted date objects for readability
				atime: stats.atime,
				mtime: stats.mtime,
				ctime: stats.ctime,
				birthtime: stats.birthtime
			};
		} catch (error) {
			console.error(error);
			throw new Error('Could not get DB info. ' + error);
		}
	}

	async getRawPTN(id: number): Promise<any> {
		try {
			const result = await this.repository.findOne({
				where: { id }
			});
			if (!result) {
				return new NotFoundException();
			}
			const ptn = this.ptnService.getPTN(result);
			return ptn;
		} catch (error) {
			console.error(error);
			throw new Error(error);
		}
	}
}
