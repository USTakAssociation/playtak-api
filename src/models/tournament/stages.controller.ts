import { Body, Controller, Get, Logger, ParseIntPipe, Put, Query, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { CreateStageDto, StageDto } from './dto/stage.dto';
import { StagesService } from './services/stages.service';

@ApiTags('Tournaments', 'TournamentStages')
@Controller({
	path: 'tournaments/stages',
	version: ['1'],
})
export class StagesController {
	private readonly logger = new Logger(StagesController.name);

	constructor(private readonly service: StagesService) {}

	@ApiOperation({ operationId: 'stages_get', summary: 'Get stage by id' })
	@ApiResponse({
		status: 200,
		type: StageDto,
		description: 'The requested stage.',
	})
	@Get(':id')
	get(@Query('id', ParseIntPipe) id: number): Promise<StageDto> {
		return this.service.getById(id);
	}

	@ApiOperation({ operationId: 'stages_create', summary: 'Create a new stage' })
	@ApiResponse({
		status: 200,
		type: StageDto,
		description: 'Returns the new stage.',
	})
	@Put()
	create(@Body(ValidationPipe) stage: CreateStageDto): Promise<StageDto> {
		return this.service.create(stage);
	}
}
