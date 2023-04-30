import { Controller, Get, Param, Post, Query } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiResponse, ApiTags } from '@nestjs/swagger';
import { TournamentsService } from './tournaments.service';
import { DefaultExceptionDto } from './dto/error.dto';
import { Tournament, TournamentsList, TournamentsQuery } from './dto/tournaments.dto';

@ApiTags('Tournaments')
@Controller({
	path: 'tournaments',
	version: ['1'],
})
export class TournamentsController {
	constructor(private readonly service: TournamentsService) {}

	@ApiOperation({ operationId: 'tournaments_list', summary: 'Get Tournaments List' })
	@ApiResponse({
		status: 200,
		type: TournamentsList,
		description: 'Returns list of tournaments',
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
	@Post()
	findAll(@Query() query: TournamentsQuery): Promise<TournamentsList> {
		return this.service.getAll(query);
	}

	@ApiOperation({ operationId: 'tournament', summary: 'Get Tournament by ID' })
	@ApiResponse({
		status: 200,
		type: Tournament,
		description: 'Returns tournament by id',
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
	getOneById(@Param('id') id: number): Promise<Tournament> {
		return this.service.getOneByID(+id);
	}
}
