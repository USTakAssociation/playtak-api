import { Controller, Get, Logger, NotFoundException, Param, ParseIntPipe, Put } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiResponse, ApiTags } from '@nestjs/swagger';
import { DefaultExceptionDto } from './dto/error.dto';
import { SeekDto } from './dto/seek.dto';
import { SeeksService } from './services/seeks.service';

@ApiTags('Tournaments', 'Seek')
@Controller({
	path: 'tournaments/seeks',
	version: ['1'],
})
export class SeeksController {
	private readonly logger = new Logger(SeeksController.name);

	constructor(private readonly service: SeeksService) {}

	@ApiOperation({ operationId: 'seek_list', summary: 'Get a list of all current seeks on the playtak server' })
	@ApiResponse({
		status: 200,
		type: Array<SeekDto>,
		description: 'Returns list of currently existing seeks',
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
	getAll(): Promise<Array<SeekDto>> {
		return this.service.getSeeks();
	}

	@ApiOperation({ operationId: 'seek_create', summary: 'Create a seek for a tournament game on the playtak server' })
	@ApiResponse({
		status: 200,
		type: SeekDto,
		description: 'The created seek',
	})
	@ApiResponse({
		status: 404,
		type: NotFoundException,
		description: 'Returns 500 server error',
	})
	@ApiParam({ name: 'gameId', description: 'The game for which to create the seek' })
	@Put(':gameId')
	createSeek(@Param('gameId', ParseIntPipe) gameId: number): Promise<SeekDto> {
		return this.service.createSeek(gameId);
	}
}
