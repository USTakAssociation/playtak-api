import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Like, MoreThan, Repository } from 'typeorm';
import { Games } from './entities/games.entity';
import {stat} from 'fs/promises';
@Injectable()
export class GamesService {
	constructor(
		@InjectRepository(Games)
		private repository: Repository<Games>,
	) {}

	async getAll(query?): Promise<any> {
		const limit = parseInt(query.limit) || 50;
		const skip = parseInt(query.skip) || 0;
		const page = parseInt(query.page) || 0;
		const order: "ASC" | "DESC" = query.order || 'DESC';
		const search: any = query.search
			? JSON.parse(decodeURI(query.search))
			: {};
		const mirror: boolean = search.mirror || false;
		delete search.mirror;
		delete search.type;

		let player_search: boolean;
		const playerWhite = search['player_white'];
		const playerBlack = search['player_black']; 
		if (playerWhite) {
			search.player_white = Like(`${playerWhite}`);
			player_search = true;
		}
		if (playerBlack) {
			search.player_black = Like(`${playerBlack}`);
			player_search = true;
		}

		if(search?.normal === 1){
			search["tournament"] = 0;
			search["unrated"] = 0;
			delete search.normal;
		}

		if(search?.game_result ){
			if (search.game_result === 'X-0') {
				search['result'] = Like('%-0');
			} else if(search.game_result === '0-X'){
				search['result'] = Like('0-%');
			} else{
				search['result'] = search.game_result;
			}
		}

		const mirrorSearch = {...search};
		if(mirror) {
			delete mirrorSearch['player_black'];
			delete mirrorSearch['player_white'];
			if (playerWhite) {
				mirrorSearch['player_black'] = Like(`${playerWhite}`);
				player_search = true;
			}
			if(playerBlack){
				mirrorSearch['player_white'] = Like(`${playerBlack}`);
				player_search = true;
			}
			if(search['game_result']) {
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
						mirrorSearch['result'] = search.game_result;
						break;
				}
			}
		}
		delete search['game_result'];
		delete mirrorSearch['game_result'];
		if(player_search){
			search['date'] = MoreThan("1461430800000");
			if(mirror) {
				mirrorSearch['date'] = MoreThan('1461430800000');
			}
		}

		try {
			let dbQuery;
			if (mirror) {
				
				dbQuery = this.repository
					.createQueryBuilder()
					.select('*')
					.where(search)
					.orWhere(mirrorSearch)
					.orderBy('id', order);
			} else {
				dbQuery = this.repository
					.createQueryBuilder()
					.select('*')
					.where(search)
					.orderBy('id', order);
			}
			const total = await dbQuery.getCount();
			const result = await dbQuery
				.clone()
				.limit(limit)
				.offset(limit * page || skip)
				.execute();
			
			for (let i = 0; i < result.length; i++) {
				const element = result[i];
				if(element.date <= 1461430800000) {
					element.player_black = "Anon";
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
	
	async getOneByID(id: number): Promise<any>{
		try {
			const result = await this.repository.findOne({
				where: {id}
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
			const result = await stat(process.env.ANON_DB_PATH);
			return result;
		} catch (error) {
			console.error(error);
			throw new Error('Could not get DB info. ' + error);
		}
	}
}
