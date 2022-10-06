import { HttpException, Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Like, Repository } from 'typeorm';
import { Games } from './entities/games.entity';

@Injectable()
export class GameHistoryService {
	constructor(
		@InjectRepository(Games)
		private repository: Repository<Games>,
	) {}
	
	async getAll(query?): Promise<any> {
		try {
			const limit = parseInt(query.limit) || 100;
			const skip = parseInt(query.skip) || 0;
			const page = parseInt(query.page) || 0;
			const column = query.by || 'id';
			const keyword = query.keyword || '';
			const [result, total] = await this.repository.findAndCount({
				where: { [column]: Like('%' + keyword + '%') },
				order: { [column]: 'DESC' },
				take: limit,
				skip: limit * page || skip,
			});
			return {
				items: result || [],
				count: total || 0,
				page: page + 1,
				totalPages: Math.ceil(total / limit),
			};
		} catch (error) {
			throw new HttpException(error.response, error.status);
		}
	}

	async getOneById(id: number): Promise<any> {
		try {
			const result = await this.repository.findOneBy({
				id
			});
			if (!result) {
				throw new HttpException('No game found by Id', 404);
			}
			return result;
		} catch (error) {
			throw new HttpException(error.response, error.status);
		}
	}

}
