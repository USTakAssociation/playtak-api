import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { stat } from 'fs/promises';
import { Between, In, LessThan, Like, MoreThan, Repository } from 'typeorm';
import { GameQuery } from '../dto/games/games.dto';
import { Games } from './entities/games.entity';
import { PTNService } from './services/ptn.service';

@Injectable()
export class GamesService {
	private static readonly SORTABLE_COLUMNS = new Set([
		'id',
		'date',
		'size',
		'player_white',
		'player_black',
		'notation',
		'result',
		'timertime',
		'timerinc',
		'rating_white',
		'rating_black',
		'unrated',
		'tournament',
		'komi',
		'pieces',
		'capstones',
		'rating_change_white',
		'rating_change_black',
		'extra_time_amount',
		'extra_time_trigger',
		'increment_scales',
		'opening'
	]);

	private static readonly TYPE_FILTERS = new Set(['normal', 'tournament', 'unrated']);

	constructor(
		@InjectRepository(Games, 'games')
		private repository: Repository<Games>,
		private ptnService: PTNService
	) {}

	// Query params are always strings at runtime, but a couple of GameQuery fields (e.g. size)
	// are typed as number — accept both so callers don't need to know which.
	// Returns undefined for anything that isn't a plain integer, instead of letting NaN
	// silently flow into a TypeORM search value.
	private parseIntStrict(value: string | number): number | undefined {
		const str = String(value);
		if (!/^-?\d+$/.test(str)) {
			return undefined;
		}
		return parseInt(str, 10);
	}

	private clampInt(value: number, min: number, max = Number.MAX_SAFE_INTEGER): number {
		return Math.min(Math.max(value, min), max);
	}

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
		// validateIdQuery already rejects mixed separators, repeated separators and
		// reversed ranges, so an id that gets past it needs no further repair. An id
		// that does not is dropped rather than parsed into a NaN search value.
		if (query['id'] && this.validateIdQuery(query['id'])) {
			if (query['id'].includes(',')) {
				search['id'] = In(query['id'].split(',').map((id) => parseInt(id, 10)));
			} else if (query['id'].includes('-')) {
				const [from, to] = query['id'].split('-').map((id) => parseInt(id, 10));
				search['id'] = Between(from, to);
			} else {
				search['id'] = parseInt(query['id'], 10);
			}
		}
		// date query handling for single value, between, greater than, less than
		if (query['date']) {
			if (query['date'].includes('-')) {
				// remove duplicate hyphens
				query['date'] = query['date'].replace(/-{2,}/g, '-');
				const dates = query['date'].split('-');
				const from = this.parseIntStrict(dates[0]);
				const to = this.parseIntStrict(dates[1]);
				if (from !== undefined && to !== undefined) {
					search['date'] = from > to ? Between(to, from) : Between(from, to);
				}
			} else if (query['date'].startsWith('>')) {
				const dateValue = this.parseIntStrict(query['date'].substring(1));
				if (dateValue !== undefined) {
					search['date'] = MoreThan(dateValue);
				}
			} else if (query['date'].startsWith('<')) {
				const dateValue = this.parseIntStrict(query['date'].substring(1));
				if (dateValue !== undefined) {
					search['date'] = LessThan(dateValue);
				}
			} else {
				const dateValue = this.parseIntStrict(query['date']);
				if (dateValue !== undefined) {
					search['date'] = dateValue;
				}
			}
		}
		if (query['player_white']) {
			search['player_white'] = query['player_white'];
		}
		if (query['player_black']) {
			search['player_black'] = query['player_black'];
		}
		// game_result/result are bound as parameterized WHERE *values* below (Like()/equality),
		// never as column or identifier names, so they need no allowlisting here.
		if (query['game_result']) {
			search['game_result'] = query['game_result'];
		}
		if (query['size']) {
			const size = this.parseIntStrict(query['size']);
			if (size !== undefined) {
				search['size'] = size;
			}
		}
		if (query['timertime']) {
			const timertime = this.parseIntStrict(query['timertime']);
			if (timertime !== undefined) {
				search['timertime'] = timertime;
			}
		}
		if (query['timerinc']) {
			const timerinc = this.parseIntStrict(query['timerinc']);
			if (timerinc !== undefined) {
				search['timerinc'] = timerinc;
			}
		}
		if (query['extra_time_amount']) {
			const extraTimeAmount = this.parseIntStrict(query['extra_time_amount']);
			if (extraTimeAmount !== undefined) {
				search['extra_time_amount'] = extraTimeAmount;
			}
		}
		if (query['extra_time_trigger']) {
			const extraTimeTrigger = this.parseIntStrict(query['extra_time_trigger']);
			if (extraTimeTrigger !== undefined) {
				search['extra_time_trigger'] = extraTimeTrigger;
			}
		}
		if (query['increment_scales']) {
			const incrementScales = this.parseIntStrict(query['increment_scales']);
			if (incrementScales !== undefined) {
				search['increment_scales'] = incrementScales;
			}
		}
		if (query['type'] && GamesService.TYPE_FILTERS.has(query['type'].toLowerCase())) {
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
			search['player_white'] = Like(`${playerWhite}`);
			player_search = true;
		}
		if (playerBlack) {
			search['player_black'] = Like(`${playerBlack}`);
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
				mirrorSearch['player_black'] = Like(`${playerWhite}`);
				player_search = true;
			}
			if (playerBlack) {
				mirrorSearch['player_white'] = Like(`${playerBlack}`);
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
		if (player_search) {
			search['date'] = MoreThan('1461430800000');
			if (mirror) {
				mirrorSearch['date'] = MoreThan('1461430800000');
			}
		}

		return { search, mirrorSearch };
	}

	async getAll(query?: GameQuery): Promise<any> {
		// sort/order are concatenated directly into the generated SQL's ORDER BY clause
		// (createQueryBuilder().select('*') means TypeORM can't resolve them against
		// entity metadata, so they go out unescaped) — must be allowlisted, not just parsed.
		const limit = this.clampInt(this.parseIntStrict(query.limit) ?? 50, 1, 200);
		const skip = this.clampInt(this.parseIntStrict(query.skip) ?? 0, 0);
		const page = this.clampInt(this.parseIntStrict(query.page) ?? 0, 0);
		const order: 'ASC' | 'DESC' = String(query.order).toUpperCase() === 'ASC' ? 'ASC' : 'DESC';
		const sort = query.sort && GamesService.SORTABLE_COLUMNS.has(query.sort) ? query.sort : 'id';
		const mirror = query.mirror === 'true' ? true : false;
		const { search, mirrorSearch } = this.generateSearchQuery(query);
		try {
			let dbQuery;
			if (mirror) {
				dbQuery = this.repository
					.createQueryBuilder()
					.select('*')
					.where(search)
					.orWhere(mirrorSearch)
					.orderBy(sort, order);
			} else {
				dbQuery = this.repository.createQueryBuilder().select('*').where(search).orderBy(sort, order);
			}

			const total = await dbQuery.getCount();
			const result = await dbQuery
				.clone()
				.limit(limit)
				.offset(limit * page || skip)
				.execute();

			return {
				items: result || [],
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
