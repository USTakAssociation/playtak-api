import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { stat } from 'fs/promises';
import { Between, In, LessThan, Like, MoreThan, Raw, Repository } from 'typeorm';
import { GameQuery } from '../dto/games/games.dto';
import { Games } from './entities/games.entity';
import { PTNService } from './services/ptn.service';

@Injectable()
export class GamesService {
	constructor(
		@InjectRepository(Games, 'games')
		private repository: Repository<Games>,
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
		// Case-insensitive match (usernames are case-insensitive identities on the
		// server -- Player.uniqifyName). `= ? COLLATE NOCASE` not LIKE: same rows,
		// but `=` is a point not a range, so the NOCASE index can also serve
		// ORDER BY id (no temp b-tree). Keep LIKE when the value has a real wildcard.
		const playerMatch = (value: string, param: string) =>
			/[%_]/.test(value)
				? Like(`${value}`)
				: Raw((col) => `${col} = :${param} COLLATE NOCASE`, { [param]: value });
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
		// Exclude pre-launch games from every player search (the old cutoff; kept
		// as-is here -- swapping it for an id bound is a separate change).
		if (player_search) {
			search['date'] = MoreThan('1461430800000');
			if (mirror) {
				mirrorSearch['date'] = MoreThan('1461430800000');
			}
		}

		return { search, mirrorSearch, playerWhite, playerBlack };
	}

	async getAll(query?: GameQuery): Promise<any> {
		const limit = parseInt(query.limit) || 50;
		const skip = parseInt(query.skip) || 0;
		const page = parseInt(query.page) || 0;
		const order: 'ASC' | 'DESC' = query.order || 'DESC';
		const sort = query.sort ? query.sort : 'id';
		const mirror = query.mirror === 'true' ? true : false;
		const { search, mirrorSearch, playerWhite, playerBlack } = this.generateSearchQuery(query);
		const offset = limit * page || skip;
		try {
			// Fast path: mirror search for one wildcard-free player, default id sort
			// (the Table.vue player links, and most typed searches). Each colour arm
			// is a point lookup the NOCASE index serves in id order, so per-arm
			// ORDER BY id is a plain index walk; merge in SQL. `A OR B` instead sorts
			// every match through a temp b-tree -- 100s of ms for a bot. Anything
			// else (wildcard, extra filter, explicit id, non-id sort) falls through.
			const onePlayer = playerWhite && !playerBlack ? playerWhite : playerBlack && !playerWhite ? playerBlack : null;
			const nonFloorKeys = (o: object) => Object.keys(o).filter((k) => k !== 'date');
			if (
				mirror &&
				sort === 'id' &&
				order === 'DESC' &&
				onePlayer &&
				!/[%_]/.test(onePlayer) &&
				nonFloorKeys(search).length === 1 &&
				nonFloorKeys(mirrorSearch).length === 1
			) {
				const FLOOR = '1461430800000';
				const armLimit = offset + limit;
				const items = await this.repository.query(
					`SELECT * FROM (
						SELECT * FROM (SELECT * FROM games WHERE player_white = ? COLLATE NOCASE AND date > ? ORDER BY id DESC LIMIT ?)
						UNION
						SELECT * FROM (SELECT * FROM games WHERE player_black = ? COLLATE NOCASE AND date > ? ORDER BY id DESC LIMIT ?)
					) ORDER BY id DESC LIMIT ? OFFSET ?;`,
					[onePlayer, FLOOR, armLimit, onePlayer, FLOOR, armLimit, limit, offset]
				);
				// Count via the same UNION, not `WHERE a = ? OR b = ?`: SQLite's
				// MULTI-INDEX OR doesn't fire for an OR of two bound params, so that
				// form scans (~240ms). UNION dedupes a player's self-games.
				const [{ total }] = await this.repository.query(
					`SELECT COUNT(*) AS total FROM (
						SELECT id FROM games WHERE player_white = ? COLLATE NOCASE AND date > ?
						UNION
						SELECT id FROM games WHERE player_black = ? COLLATE NOCASE AND date > ?
					);`,
					[onePlayer, FLOOR, onePlayer, FLOOR]
				);
				return {
					items: items || [],
					total: total || 0,
					page: page + 1,
					perPage: limit,
					totalPages: Math.ceil(total / limit)
				};
			}

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
			const result = await dbQuery.clone().limit(limit).offset(offset).execute();

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
