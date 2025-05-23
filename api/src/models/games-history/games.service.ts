import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Between, In, Like, MoreThan, Repository } from 'typeorm';
import { Games } from './entities/games.entity';
import { stat } from 'fs/promises';
import { PTNService } from './services/ptn.service';
import { GameQuery } from '../dto/games/games.dto';
@Injectable()
export class GamesService {
	constructor(
		@InjectRepository(Games, 'games')
		private repository: Repository<Games>,
		private ptnService: PTNService,
	) {}

	validateIdQuery(id: string) {
		const regex = /^(?!.*,,)(?!.*--)\d+([-,\d]*\d+)?$/;
		if (!regex.test(id)) {
			return false;
		}
		
		// cannot contain both a hyphen and a comma
		if (id.includes("-") && id.includes(",")) {
			return false;
		}
		// if value has a hyphen check that the second number is greater than the first
		if (id.includes("-")) {
			const idArray = id.split("-");
			if (parseInt(idArray[0]) >= parseInt(idArray[1])) {
				return false;
			}
		}
		return true;
	}

	generateSearchQuery(query: GameQuery) {
		const search = {};
		query['id'] ? (search['id'] = parseInt(query['id'])) : null;
		if( query['id'] && this.validateIdQuery(query['id']) ) {
			if(query['id'].includes(',')) {
				const ids = query['id'].split(',');
				const arr = [];
				for(let i = 0; i < ids.length; i++) {
					arr.push(parseInt(ids[i]));
				}
				search['id'] = In(arr);
			}
			if(query['id'].includes('-')) {
				const ids = query['id'].split('-');
				search['id'] = Between(parseInt(ids[0]), parseInt(ids[1]))
			}
		}
		if(query['id'] && query['id'].includes('-')) {
			// remove duplicate hyphens
			query['id'] = query['id'].replace(/-{2,}/g, '-');
			const ids = query['id'].split('-');
			// make sure the first id is smaller than the second
			if(parseInt(ids[0]) > parseInt(ids[1])) {
				const temp = ids[0];
				ids[0] = ids[1];
				ids[1] = temp;
			}
			search['id'] = Between(parseInt(ids[0]), parseInt(ids[1]))
		}
		query['player_white']
			? (search['player_white'] = query['player_white'])
			: null;
		query['player_black']
			? (search['player_black'] = query['player_black'])
			: null;
		query['game_result']
			? (search['game_result'] = query['game_result'])
			: null;
		query['size'] ? (search['size'] = query['size']) : null;
		query['type'] ? (search[query['type'].toLowerCase()] = 1) : null;
		const mirror = query.mirror === 'true' ? true : false;

		if (search['normal']) {
			search['tournament'] = 0;
			search['unrated'] = 0;
			delete search['normal']
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
		
		return {search, mirrorSearch};
	}

	async getAll(query?: GameQuery): Promise<any> {
		const limit = parseInt(query.limit) || 50;
		const skip = parseInt(query.skip) || 0;
		const page = parseInt(query.page) || 0;
		const order: 'ASC' | 'DESC' = query.order || 'DESC';
		const sort = query.sort ? query.sort : 'id';
		const mirror = query.mirror === 'true' ? true : false;
		const {search, mirrorSearch} = this.generateSearchQuery(query);
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
				dbQuery = this.repository
					.createQueryBuilder()
					.select('*')
					.where(search)
					.orderBy(sort, order);
			}
			const total = await dbQuery.getCount();
			const result = await dbQuery
				.clone()
				.limit(limit)
				.offset(limit * page || skip)
				.execute();

			for (let i = 0; i < result.length; i++) {
				const element = result[i];
				if (element.date <= 1461430800000) {
					element.player_black = 'Anon';
					element.player_white = 'Anon';
				}
			}
			return {
				items: result || [],
				total: total || 0,
				page: page + 1,
				perPage: limit,
				totalPages: Math.ceil(total / limit),
			};
		} catch (error) {
			console.error(error);
			throw new Error('Could not get games. ' + error);
		}
	}

	async getOneByID(id: number): Promise<any> {
		try {
			const result = await this.repository.findOne({
				where: { id },
			});
			if (result && result['date'] <= 1461430800000) {
				result['player_black'] = 'Anon';
				result['player_white'] = 'Anon';
			}
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
				where: { id },
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
