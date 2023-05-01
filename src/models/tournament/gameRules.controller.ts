import { Body, Controller, Get, Logger, Put, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { CreateGameRulesDto, GameRulesDto } from './dto/gameRules.dto';
import { MatchupsQuery } from './dto/matchups.dto';
import { GameRulesService } from './services/gameRules.service';

@ApiTags('Tournaments', 'GameRules')
@Controller({
	path: 'tournaments/game-rules',
	version: ['1'],
})
export class GameRulesController {
	private readonly logger = new Logger(GameRulesController.name);

	constructor(private readonly matchupsService: GameRulesService) {}

	@ApiOperation({ operationId: 'gamerules_get_all', summary: 'Get all game-rules' })
	@ApiResponse({
		status: 200,
		type: Array<GameRulesDto>,
		description: 'A list of all game rules.',
	})
	@Get()
	getAll(@Body() query: MatchupsQuery): Promise<Array<GameRulesDto>> {
		return this.matchupsService.getAll(query);
	}

	@ApiOperation({ operationId: 'gamerules_create', summary: 'Create a new game-rule' })
	@ApiResponse({
		status: 200,
		type: GameRulesDto,
		description: 'The created game rule.',
	})
	@Put()
	createMatchup(@Body(ValidationPipe) matchup: CreateGameRulesDto): Promise<GameRulesDto> {
		return this.matchupsService.createMatchup(matchup);
	}
}
