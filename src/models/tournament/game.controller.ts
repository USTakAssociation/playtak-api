import { BadRequestException, Body, Controller, Logger, NotImplementedException, Post } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { TournamentsService } from './tournaments.service';
import { DefaultExceptionDto } from './dto/error.dto';
import { GameUpdateDto } from './dto/gameUpdate.dto';

@ApiTags('Tournaments', 'TournamentGame')
@Controller({
	path: 'tournaments/game',
	version: ['1'],
})
export class GameController {
	private readonly logger = new Logger(GameController.name);

	constructor(private readonly service: TournamentsService) {}

	@ApiOperation({ operationId: 'game_update', summary: 'Receives updates about games played' })
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

		return this.service.handleGameUpdate(gameUpdate);
	}
}
