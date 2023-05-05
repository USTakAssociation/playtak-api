import { Body, Controller, Get, Logger, Put, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { CreateGameRulesDto, GameRulesDto, GameRulesQuery } from '../dto/gameRules.dto';
import { GameRulesService } from '../services/gameRules.service';

@ApiTags('Tournaments', 'GameRules')
@Controller({
	path: 'tournaments/game-rules',
	version: ['1'],
})
export class GameRulesController {
	private readonly logger = new Logger(GameRulesController.name);

	constructor(private readonly service: GameRulesService) {}

	@ApiOperation({ operationId: 'gamerules_get_all', summary: 'Get all game-rules' })
	@ApiResponse({
		status: 200,
		type: Array<GameRulesDto>,
		description: 'A list of all game-rules.',
	})
	@Get()
	getAll(@Body() query: GameRulesQuery): Promise<Array<GameRulesDto>> {
		return this.service.getAll(query);
	}

	@ApiOperation({ operationId: 'gamerules_create', summary: 'Create a new game-rule' })
	@ApiResponse({
		status: 200,
		type: GameRulesDto,
		description: 'The created game rule.',
	})
	@Put()
	createGameRule(@Body(ValidationPipe) matchup: CreateGameRulesDto): Promise<GameRulesDto> {
		return this.service.createGameRules(matchup);
	}
}
