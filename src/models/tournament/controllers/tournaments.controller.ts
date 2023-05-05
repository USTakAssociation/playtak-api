import { Body, Controller, Get, Logger, Param, ParseIntPipe, Post, Put, Query, ValidationPipe } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiResponse, ApiTags } from '@nestjs/swagger';
import { DefaultExceptionDto } from '../../common/dto/error.dto';
import { CreateTournamentDto, TournamentDto, TournamentsList, TournamentsQuery } from '../dto/tournaments.dto';
import { TournamentsService } from '../services/tournaments.service';

@ApiTags('Tournaments')
@Controller({
	path: 'tournaments',
	version: ['1'],
})
export class TournamentsController {
	private readonly logger = new Logger(TournamentsController.name);

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

	@ApiOperation({ operationId: 'tournament', summary: 'Get the entire tournament with all its stages etc. by ID' })
	@ApiResponse({
		status: 200,
		type: TournamentDto,
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
	@ApiParam({ name: 'id', description: 'Tournament ID' })
	@Get(':id')
	getOneById(@Param('id', ParseIntPipe) id: number): Promise<TournamentDto> {
		return this.service.getEntireTournamentById(id);
	}

	@ApiOperation({ operationId: 'tournaments_create', summary: 'Create a new tournament' })
	@ApiResponse({
		status: 200,
		type: CreateTournamentDto,
		description: 'The created game.',
	})
	@Put()
	createGame(@Body(ValidationPipe) tournament: CreateTournamentDto): Promise<TournamentDto> {
		return this.service.createTournament(tournament);
	}
}
