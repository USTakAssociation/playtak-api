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
		
		if(search?.player_white || search?.player_black){
			player_search = true;
			if(search?.partial_user){
				if(search?.player_white){
					search.player_white = Like(`%${search.player_white}%`);
				}
				if (search?.player_black) {
					search.player_black = Like(`%${search.player_black}%`);
				}
			}
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

		const mirrorSearch = {};
		if(mirror) {
			if ( search?.player_white ) {
				mirrorSearch['player_black'] = search.player_white;
				if (search?.partial_user) {
					search.player_white = Like(`%${search.player_white}%`);
				}
				player_search = true;
			}
			
			if(search?.player_black){
				mirrorSearch['player_white'] = search.player_black;
				if (search?.player_black) {
					search.player_black = Like(`%${search.player_black}%`);
				}
				player_search = true;
			}
			
			if ( search?.game_result) {
				if (search.game_result === 'X-0') {
					mirrorSearch['result'] = Like('0-%');
				} else if (search.game_result === '0-X') {
					mirrorSearch['result'] = Like('%-0');
				} else if (search.game_result === '1/2-1/2') {
					mirrorSearch['result'] = '1/2-1/2';
				} else {
					mirrorSearch['result'] = search.game_result;
				}
			}
		}
		delete search.game_result;
		delete search.partial_user;
		if(player_search){
			search['date'] = MoreThan("1461430800000");
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
			const result = this.repository.findOne({
				where: {id}
			})
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
