import { Injectable, Logger, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { CreateGameRulesDto, GameRulesDto, GameRulesQuery } from '../dto/gameRules.dto';
import { GameRules } from '../entities/gameRules.entity';

@Injectable()
export class GameRulesService {
	private readonly logger = new Logger(GameRulesService.name);

	constructor(
		@InjectRepository(GameRules)
		private gameRules: Repository<GameRules>,
	) {}

	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	async getAll(query: GameRulesQuery): Promise<Array<GameRulesDto>> {
		return await this.gameRules.find();
	}

	async getById(id: number): Promise<GameRules> {
		try {
			return await this.gameRules.findOneByOrFail({ id });
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no GameRules with id='${id}'`, { cause });
				this.logger.warn(exc)
				throw exc;
			}
			throw cause;
		}
	}
	
	async createGameRules(matchupToCreate: CreateGameRulesDto): Promise<GameRulesDto> {
		const gameRulesInstance = this.gameRules.create({
			...matchupToCreate,
			id: undefined,
		});
		return await this.gameRules.manager.save<GameRules>(gameRulesInstance);
	}
}
