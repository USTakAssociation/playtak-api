import { Body, Controller, Get, Logger, Param, ParseIntPipe, Put, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiResponse, ApiTags } from '@nestjs/swagger';
import { CreateMatchupDto, MatchupDto } from '../dto/matchups.dto';
import { MatchupsService } from '../services/matchups.service';

@ApiTags('Tournaments', 'TournamentMatchups')
@Controller({
	path: 'tournaments/matchups',
	version: ['1'],
})
export class MatchupsController {
	private readonly logger = new Logger(MatchupsController.name);

	constructor(private readonly service: MatchupsService) {}

	@ApiOperation({ operationId: 'matchups_get', summary: 'Get matchup by id' })
	@ApiResponse({
		status: 200,
		type: MatchupDto,
		description: 'The requested matchup.',
	})
	@ApiParam({ name: 'id', description: 'Matchup ID' })
	@Get(':id')
	get(@Param('id', ParseIntPipe) id: number): Promise<MatchupDto> {
		return this.service.getById(id);
	}

	@ApiOperation({ operationId: 'matchups_create', summary: 'Create a new matchup' })
	@ApiResponse({
		status: 200,
		type: MatchupDto,
		description: 'Returns the new matchup.',
	})
	@Put()
	create(@Body(ValidationPipe) matchup: CreateMatchupDto): Promise<MatchupDto> {
		return this.service.create(matchup);
	}
}