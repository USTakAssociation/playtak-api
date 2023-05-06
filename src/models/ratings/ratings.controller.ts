import { Controller, Get, Param, Query } from '@nestjs/common';
import { ApiOperation, ApiParam, ApiResponse, ApiTags } from '@nestjs/swagger';
import { RatingService } from './ratings.service';
import { DefaultExceptionDto } from './dto/error.dto';
import { Rating, RatingList, RatingQuery } from './dto/ratings.dto';

@ApiTags('Ratings')
@Controller({
	path: 'ratings',
	version: ['1'],
})
export class RatingsController {
	constructor(private readonly service: RatingService) {}

	@ApiOperation({ operationId: 'rating_list', summary: 'Get Rating List' })
	@ApiResponse({status: 200, type: RatingList, description: 'Returns list of games' })
	@ApiResponse({status: 404, type: DefaultExceptionDto, description: 'Returns 404 server error'})
	@ApiResponse({status: 429, type: DefaultExceptionDto, description: 'Returns 429 too many requests error'})
	@ApiResponse({status: 500, type: DefaultExceptionDto, description: 'Returns 500 server error'})
	@Get()
	findAll(@Query() query: RatingQuery): Promise<RatingList> {
		return this.service.getAll(query);
	}
	
	@ApiOperation({ operationId: 'Player Rating', summary: 'Get Players Rating' })
	@ApiResponse({status: 200, type: RatingList, description: 'Returns list of games' })
	@ApiResponse({status: 404, type: DefaultExceptionDto, description: 'Returns 404 server error'})
	@ApiResponse({status: 429, type: DefaultExceptionDto, description: 'Returns 429 too many requests error'})
	@ApiResponse({status: 500, type: DefaultExceptionDto, description: 'Returns 500 server error'})
	@ApiParam({ name: 'name', description: 'player name', required: true })
	@Get('/:name')
	findRating(@Param() param: any): Promise<Rating> {
		return this.service.getPlayersRating(param.name);
	}
}
