import { Injectable, Logger, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { CreateMatchupDto, MatchupDto } from '../dto/matchups.dto';
import { Matchup } from '../entities/matchup.entity';
import { Group } from '../entities/group.entity';
@Injectable()
export class MatchupsService {
	private readonly logger = new Logger(MatchupsService.name);

	constructor(
		@InjectRepository(Matchup)
		private matchups: Repository<Matchup>,
		@InjectRepository(Group)
		private groups: Repository<Group>,
	) {}


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
	
	async create(matchup: CreateMatchupDto): Promise<MatchupDto> {
		return await this.matchups.manager.save<Matchup>(this.matchups.create({
			...matchup,
			group: await this.groups.findOneByOrFail({ id: matchup.group }),
			games: []
		}));
	}
}
