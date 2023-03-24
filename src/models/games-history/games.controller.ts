import { Controller, Get, Param, Query } from '@nestjs/common';
import { GamesService } from './games.service';

@Controller('games-history')
export class GamesController {
	constructor(private readonly service: GamesService) {}

	@Get()
	findAll(@Query() query) {
		return this.service.getAll(query);
	}

	@Get(':id')
	getOneById(@Param('id') id: number) {
		return this.service.getOneByID(+id);
	}

	@Get('ptn/:id')
	getPTN(@Param('id') id: number) {
		return this.service.getRawPTN(+id);
	}

	@Get('/db')
	getDBInfo() {
		return this.service.getDBInfo();
	}
}
