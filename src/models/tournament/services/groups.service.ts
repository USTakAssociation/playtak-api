import { Injectable, Logger, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { CreateGroupDto, GroupDto } from '../dto/group.dto';
import { Group } from '../entities/group.entity';
import { Stage } from '../entities/stage.entity';
@Injectable()
export class GroupsService {
	private readonly logger = new Logger(GroupsService.name);

	constructor(
		@InjectRepository(Group)
		private groups: Repository<Group>,
		@InjectRepository(Stage)
		private stages: Repository<Stage>,
	) {}


	async getById(id: number): Promise<Group> {
		try {
			this.logger.log("get by id", id)
			return await this.groups.findOneByOrFail({ id });
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
	
	async create(groupToCreate: CreateGroupDto): Promise<GroupDto> {
		return await this.groups.manager.save<Group>(this.groups.create({
			...groupToCreate,
			stage: await this.stages.findOneByOrFail({ id: groupToCreate.stage }),
			matchups: []
		}));
	}
}
