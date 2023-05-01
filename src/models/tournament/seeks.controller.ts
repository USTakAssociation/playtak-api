import { Controller, Get, Logger } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { DefaultExceptionDto } from './dto/error.dto';
import { SeekDto } from './dto/seek.dto';
import { TournamentsList } from './dto/tournaments.dto';
import { SeeksService } from './services/seeks.service';

@ApiTags('Tournament', 'Seek')
@Controller({
	path: 'tournaments/seeks',
	version: ['1'],
})
export class SeeksController {
	private readonly logger = new Logger(SeeksController.name);

	constructor(private readonly service: SeeksService) {}

	@ApiOperation({ operationId: 'seek_list', summary: 'Get a list of all current seeks' })
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
	findAll(): Promise<Array<SeekDto>> {
		return this.service.getSeeks();
	}
}
