import { Injectable, Logger, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { Tournament } from '../entities/tournament.entity';
import { CreateStageDto, StageDto } from '../dto/stage.dto';
import { Stage } from '../entities/stage.entity';
import { GameRulesService } from './gameRules.service';
@Injectable()
export class StagesService {
	private readonly logger = new Logger(StagesService.name);

	constructor(
		@InjectRepository(Stage)
		private stages: Repository<Stage>,
		@InjectRepository(Tournament)
		private tournaments: Repository<Tournament>,
		private gameRulesService: GameRulesService,
	) {}


	async getById(id: number): Promise<Stage> {
		try {
			this.logger.log("get by id", id)
			return await this.stages.findOneByOrFail({ id });
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no Stage with id='${id}'`, { cause });
				this.logger.warn(exc)
				throw exc;
			}
			throw cause;
		}
	}
	
	async create(stage: CreateStageDto): Promise<StageDto> {
		return await this.stages.manager.save<Stage>(this.stages.create({
			...stage,
			tournament: await this.tournaments.findOneByOrFail({ id: stage.tournament }),
			rules: await this.gameRulesService.getById(stage.rules),
			groups: [],
		}));
	}
}
