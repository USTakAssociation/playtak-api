import { Controller, Get, Param, Query } from '@nestjs/common';
import { GamesService } from './games.service';

@Controller('games')
export class GamesController {
	constructor(private readonly service: GamesService) {}

	@Get()
	findAll(@Query() query) {
		return this.service.getAll(query);
	}

	@Get(':id')
	getOneById(@Param('id') id: number) {
		console.log(id)
		return this.service.getOneByID(+id);
	}
	
	@Get('/db')
	getDBInfo(){
		return this.service.getDBInfo();
	}
}
