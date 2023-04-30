import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Tournament } from './entities/tournament.entity';
import { TournamentsList, TournamentsQuery } from './dto/tournaments.dto';
@Injectable()
export class TournamentsService {
	constructor(
		@InjectRepository(Tournament)
		private repository: Repository<Tournament>,
	) {}

	generateSearchQuery(query: TournamentsQuery) {
		const search = {};
		const finished: boolean = query['finished'];
		if (finished == true || finished == false) {
			search['finished'] == finished
		}
		
		return search;
	}

	async getAll(query?: TournamentsQuery): Promise<TournamentsList> {
		const search =this.generateSearchQuery(query);
		try {
			const dbQuery = this.repository
				.createQueryBuilder()
				.select('*')
				.where(search)
				.orderBy('id');
			const result = await dbQuery.execute();

			return {
				tournaments: result ?? [],
			};
		} catch (error) {
			console.error(error);
			throw new Error('Could not get tournaments. ' + error);
		}
	}

	async getOneByID(id: number): Promise<Tournament> {
		if (!Number.isSafeInteger(id)) {
			throw new Error(`id must be an integer, but was '${id}'`);
		}
		return await this.repository
			.findOneByOrFail({ id });
	}
}
