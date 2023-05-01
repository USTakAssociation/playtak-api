import { Injectable, Logger, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { CreateMatchupDto, MatchupDto, MatchupsQuery } from '../dto/matchups.dto';
import { Matchup } from '../entities/matchup.entity';
@Injectable()
export class MatchupsService {
	private readonly logger = new Logger(MatchupsService.name);

	constructor(
		@InjectRepository(Matchup)
		private matchups: Repository<Matchup>,
	) {}

	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	async getAll(query: MatchupsQuery): Promise<Array<MatchupDto>> {
		return await this.matchups.find();
	}

	async getById(id: number): Promise<Matchup> {
		try {
			this.logger.log("get by id", id)
			return await this.matchups.findOneByOrFail({ id });
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no Matchup with id='${id}'`, { cause });
				this.logger.warn(exc)
				throw exc;
			}
			throw cause;
		}
	}
	
	async createMatchup(matchupToCreate: CreateMatchupDto): Promise<MatchupDto> {
		// todo check matchup.group exists

		const matchup2 = this.matchups.create({
			...matchupToCreate,
			group: null,
		});
		const matchup = await this.matchups.manager.save<Matchup>(matchup2);
		return matchup;
	}
}
