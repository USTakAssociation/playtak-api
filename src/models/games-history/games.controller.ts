import { Controller, Get, Param, ParseIntPipe, Query } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiResponse, ApiTags } from '@nestjs/swagger';
import { GamesService } from './games.service';
import { DefaultExceptionDto } from './dto/error.dto';
import { AnonDetails, Game, GameQuery, GamesList } from './dto/games.dto';

@ApiTags('Games History')
@Controller({
	path: 'games-history',
	version: ['1'],
})
export class GamesController {
	constructor(private readonly service: GamesService) {}

	@ApiOperation({ operationId: 'games_list', summary: 'Get Games List' })
	@ApiResponse({
		status: 200,
		type: GamesList,
		description: 'Returns list of games',
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error',
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error',
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error',
	})
	@Get()
	findAll(@Query() query: GameQuery): Promise<GamesList> {
		return this.service.getAll(query);
	}

	@ApiOperation({ operationId: 'game', summary: 'Get Game by ID' })
	@ApiResponse({
		status: 200,
		type: Game,
		description: 'Returns game by id',
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error',
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error',
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error',
	})
	@ApiParam({ name: 'id', description: 'Game ID' })
	@Get(':id')
	getOneById(@Param('id', ParseIntPipe) id: number): Promise<Game> {
		return this.service.getOneByID(+id);
	}

	@ApiOperation({ operationId: 'game-ptn', summary: 'Get raw game ptn' })
	@ApiResponse({
		status: 200,
		type: Game,
		description: 'Returns game ptn',
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error',
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error',
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error',
	})
	@Get('ptn/:id')
	getPTN(@Param('id') id: number) {
		return this.service.getRawPTN(+id);
	}

	@ApiOperation({ operationId: 'anon-db', summary: 'Get anon db details' })
	@ApiResponse({
		status: 200,
		type: AnonDetails,
		description: 'Returns anon db info',
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error',
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many requests error',
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error',
	})
	@Get('/db')
	getDBInfo() {
		return this.service.getDBInfo();
	}
}
