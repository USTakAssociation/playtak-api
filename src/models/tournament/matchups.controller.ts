import { Body, Controller, Get, Logger, Post, Put, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { MatchupsService } from './services/matchups.service';
import { CreateMatchupDto, MatchupDto, MatchupsQuery } from './dto/matchups.dto';

@ApiTags('Tournaments', 'TournamentMatchups')
@Controller({
	path: 'tournaments/matchups',
	version: ['1'],
})
export class MatchupsController {
	private readonly logger = new Logger(MatchupsController.name);

	constructor(private readonly matchupsService: MatchupsService) {}

	@ApiOperation({ operationId: 'matchups_get_all', summary: 'Get all matchups for all tournaments' })
	@ApiResponse({
		status: 200,
		type: Array<MatchupDto>,
		description: 'A list of all matchups.',
	})
	@Get()
	getAll(@Body() query: MatchupsQuery): Promise<Array<MatchupDto>> {
		return this.matchupsService.getAll(query);
	}

	@ApiOperation({ operationId: 'matchups_create', summary: 'Create a new matchup' })
	@ApiResponse({
		status: 200,
		type: MatchupDto,
		description: 'A list of all matchups.',
	})
	@Put()
	createMatchup(@Body(ValidationPipe) matchup: CreateMatchupDto): Promise<MatchupDto> {
		return this.matchupsService.createMatchup(matchup);
	}
}
