import { Body, Controller, Get, Logger, ParseIntPipe, Put, Query, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { CreateGroupDto, GroupDto } from '../dto/group.dto';
import { GroupsService } from '../services/groups.service';

@ApiTags('Tournaments', 'TournamentGroups')
@Controller({
	path: 'tournaments/groups',
	version: ['1'],
})
export class GroupsController {
	private readonly logger = new Logger(GroupsController.name);

	constructor(private readonly service: GroupsService) {}

	@ApiOperation({ operationId: 'groups_get', summary: 'Get group by id' })
	@ApiResponse({
		status: 200,
		type: GroupDto,
		description: 'The requested group.',
	})
	@Get(':id')
	get(@Query('id', ParseIntPipe) id: number): Promise<GroupDto> {
		return this.service.getById(id);
	}

	@ApiOperation({ operationId: 'groups_create', summary: 'Create a new group' })
	@ApiResponse({
		status: 200,
		type: GroupDto,
		description: 'Returns the new group.',
	})
	@Put()
	create(@Body(ValidationPipe) group: CreateGroupDto): Promise<GroupDto> {
		return this.service.create(group);
	}
}
