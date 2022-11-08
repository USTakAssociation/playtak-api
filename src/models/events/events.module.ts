import { CacheInterceptor, CacheModule, Module } from '@nestjs/common';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { EventsController } from './events.controller';
import { EventsService } from './events.service';

@Module({
	controllers: [EventsController],
	imports: [
		CacheModule.register({
			ttl: 300, // seconds
			max: 100, // maximum number of items in cache
		}),
	],
	providers: [
		EventsService,
		{
			provide: APP_INTERCEPTOR,
			useClass: CacheInterceptor,
		},
	],
})
export class EventsModule {}
