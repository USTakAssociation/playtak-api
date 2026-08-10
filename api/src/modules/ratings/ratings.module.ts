import { Module } from '@nestjs/common';
import { CacheModule, CacheInterceptor } from '@nestjs/cache-manager';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { ThrottlerModule } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import { RatingsController } from './ratings.controller';
import { RatingService } from './ratings.service';
import { Ratings } from './entities/ratings.entity';
import { RatingTask } from './tasks/rating.task';
import { Players } from './entities/players.entity';
import { Games } from '../games-history/entities/games.entity';

@Module({
	controllers: [RatingsController],
	imports: [
		TypeOrmModule.forFeature([Ratings, Players], 'default'),
		TypeOrmModule.forFeature([Games], 'games'),
		// TODO: ThrottlerModule is @Global(), and forRootAsync is called separately
		// here and in events.module.ts/games.module.ts with different limits.
		// Verify these don't clobber each other (last-imported module winning globally).
		ThrottlerModule.forRootAsync({
			useFactory: () => [
				{
					ttl: 60000,
					limit: 60
				}
			]
		}),
		CacheModule.register({
			ttl: 1800, // 30 minutes — rating refreshes every hour
			max: 100 // maximum number of items in cache
		})
	],
	providers: [
		RatingTask,
		RatingService,
		{
			provide: APP_INTERCEPTOR,
			useClass: CacheInterceptor
		}
	]
})
export class RatingsModule {}
