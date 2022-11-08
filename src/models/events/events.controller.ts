import { CacheInterceptor, Controller, Get, Header, UseInterceptors } from '@nestjs/common';
import { EventsService } from './events.service';

@Controller('events')
export class EventsController {
	constructor(private readonly service: EventsService) {}

	@Get()
	@UseInterceptors(CacheInterceptor)
	@Header('Cache-Control', 'max-age=300')
	getRMAHistory() {
		return this.service.getEvents();
	}
}
