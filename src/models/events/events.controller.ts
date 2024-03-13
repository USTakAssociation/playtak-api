import { CacheInterceptor, Controller, Get, Header, UseInterceptors } from '@nestjs/common';
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger';
import { DefaultExceptionDto } from '../common/dto/error.dto';
import { EventList } from '../games-history/dto/events.dto';
import { EventsService } from './events.service';

@ApiTags('Events')
@Controller('events')
export class EventsController {
	constructor(private readonly service: EventsService) {}

	@ApiOperation({ operationId: 'event', summary: 'Get events' })
	@ApiResponse({
		status: 200,
		type: EventList,
		description: 'Returns list of events',
	})
	@ApiResponse({
		status: 404,
		type: DefaultExceptionDto,
		description: 'Returns 404 server error',
	})
	@ApiResponse({
		status: 429,
		type: DefaultExceptionDto,
		description: 'Returns 429 too many reuests error',
	})
	@ApiResponse({
		status: 500,
		type: DefaultExceptionDto,
		description: 'Returns 500 server error',
	})
	@Get()
	@UseInterceptors(CacheInterceptor)
	@Header('Cache-Control', 'max-age=300')
	getRMAHistory() {
		return this.service.getEvents();
	}
}
