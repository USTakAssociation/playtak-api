import { BadRequestException, Body, Controller, Get, Logger, NotImplementedException, Post, Put, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { DefaultExceptionDto } from './dto/error.dto';
import { GameUpdateDto } from './dto/gameUpdate.dto';
import { CreateGameDto, GameDto, GameQuery } from './dto/game.dto';
import { GameService } from './services/game.service';

@ApiTags('Tournaments', 'TournamentGame')
@Controller({
	path: 'tournaments/game',
	version: ['1'],
})
export class GameController {
	private readonly logger = new Logger(GameController.name);

	constructor(private readonly gameService: GameService) {}

	@ApiOperation({ operationId: 'game_update', summary: 'Receives updates (e.g. from playtak-server) about games played' })
	@ApiResponse({
		status: 200,
		type: null,
		description: 'Confirmation that the update has been received',
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error',
	})
	@ApiResponse({
		status: 400,
		type: BadRequestException,
	})
	@ApiResponse({
		status: 501,
		type: NotImplementedException,
		description: 'When called with arguments not yet handled',
	})
	@Post("update")
	gameUpdate(@Body() gameUpdate: GameUpdateDto): Promise<void|NotImplementedException|TypeError> {
		// TODO: This should only allow requests from playtak - is it enough to check for a localhost IP?
		//       or do we need a secret token?

		if (!gameUpdate?.game) {
			this.logger.debug("Ignoring invalid update", gameUpdate);	
			throw new BadRequestException("GameUpdate.game must be defined")
		}

		return this.gameService.handleGameUpdate(gameUpdate);
	}


	@ApiOperation({ operationId: 'game_get_all', summary: 'Get all games for all tournaments' })
	@ApiResponse({
		status: 200,
		type: Array<GameDto>,
		description: 'A list of all games.',
	})
	@Get()
	getAll(@Body() query: GameQuery): Promise<Array<GameDto>> {
		return this.gameService.getAll(query);
	}

	
	@ApiOperation({ operationId: 'matchups_create', summary: 'Create a new game in a matchup' })
	@ApiResponse({
		status: 200,
		type: GameDto,
		description: 'The created game.',
	})
	@Put()
	createMatchup(@Body(ValidationPipe) game: CreateGameDto): Promise<GameDto> {
		return this.gameService.createGame(game);
	}
}
