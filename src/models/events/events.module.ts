import { CacheInterceptor, CacheModule, Module } from '@nestjs/common';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { ThrottlerModule } from '@nestjs/throttler';
import { EventsController } from './events.controller';
import { EventsService } from './events.service';

@Module({
	controllers: [EventsController],
	imports: [
		ThrottlerModule.forRootAsync({
			useFactory: () => ({
				ttl: 60,
				limit: 5,
			}),
		}),
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
