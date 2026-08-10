/* istanbul ignore file */
import { Module } from '@nestjs/common';
import { CacheModule, CacheInterceptor } from '@nestjs/cache-manager';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { ThrottlerModule } from '@nestjs/throttler';
import { EventsController } from './events.controller';
import { EventsService } from './events.service';

@Module({
	controllers: [EventsController],
	imports: [
		// TODO: ThrottlerModule is @Global(), and forRootAsync is called separately
		// here and in games.module.ts/ratings.module.ts with different limits.
		// Verify these don't clobber each other (last-imported module winning globally).
		ThrottlerModule.forRootAsync({
			useFactory: () => [
				{
					ttl: 60000,
					limit: 30
				}
			]
		}),
		CacheModule.register({
			ttl: 300, // seconds
			max: 100 // maximum number of items in cache
		})
	],
	providers: [
		EventsService,
		{
			provide: APP_INTERCEPTOR,
			useClass: CacheInterceptor
		}
	]
})
export class EventsModule {}
