import { Controller, Get, Param, Query } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiQuery, ApiResponse, ApiTags } from '@nestjs/swagger';
import { DefaultExceptionDto } from '../dto/error.dto';
import { AnonDetails, Game, GameQuery, GamesList } from '../dto/games/games.dto';
import { GamesService } from './games.service';

@ApiTags('Games History')
@Controller({
	path: 'games-history',
	version: ['1']
})
export class GamesController {
	constructor(private readonly service: GamesService) {}

	@ApiOperation({ operationId: 'games_list', summary: 'Get Games List' })
	@ApiResponse({
		status: 200,
		type: GamesList,
		description: 'Returns list of games'
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error'
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error'
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error'
	})
	@Get()
	findAll(@Query() query: GameQuery): Promise<GamesList> {
		return this.service.getAll(query);
	}

	@ApiOperation({ operationId: 'game', summary: 'Get Game by ID' })
	@ApiResponse({
		status: 200,
		type: Game,
		description: 'Returns game by id'
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error'
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error'
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error'
	})
	@ApiParam({ name: 'id', description: 'Game ID' })
	@Get(':id')
	getOneById(@Param('id') id: number): Promise<Game> {
		return this.service.getOneByID(+id);
	}

	@ApiOperation({ operationId: 'game-ptn', summary: 'Get raw game ptn' })
	@ApiResponse({
		status: 200,
		type: Game,
		description: 'Returns game ptn'
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error'
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error'
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error'
	})
	@ApiQuery({
		name: 'clocks',
		required: false,
		description:
			"Set to true to append each move's remaining clock as a PTN Ninja clock note, e.g. {clock1:4:32}. Intended for links that open the game in PTN Ninja."
	})
	@Get('ptn/:id')
	getPTN(@Param('id') id: number, @Query('clocks') clocks?: string) {
		return this.service.getRawPTN(+id, clocks === 'true');
	}

	@ApiOperation({ operationId: 'anon-db', summary: 'Get anon db details' })
	@ApiResponse({
		status: 200,
		type: AnonDetails,
		description: 'Returns anon db info'
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error'
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error'
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error'
	})
	@Get('/db')
	getDBInfo() {
		return this.service.getDBInfo();
	}
}
