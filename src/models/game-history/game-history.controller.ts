import { Controller, Get, Param, Query } from '@nestjs/common';
import { GameHistoryService } from './game-history.service';

@Controller('game-history')
export class GameHistoryController {
	constructor(
		private readonly service: GameHistoryService
	) {}
	@Get()
	findAll(@Query() query) {
		return this.service.getAll(query);
	}

	@Get(':id')
	findOne(@Param('id') id: string) {
		return this.service.getOneById(+id);
	}
}
