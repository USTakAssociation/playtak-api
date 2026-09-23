import { Module } from '@nestjs/common';
import { CacheModule, CacheInterceptor } from '@nestjs/cache-manager';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { ThrottlerModule } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Games } from '../games-history/entities/games.entity';
import { Players } from './entities/players.entity';
import { Ratings } from './entities/ratings.entity';
import { RatingsController } from './ratings.controller';
import { RatingService } from './ratings.service';
import { RatingTask } from './tasks/rating.task';

@Module({
	controllers: [RatingsController],
	imports: [
		TypeOrmModule.forFeature([Ratings, Players], 'default'),
		TypeOrmModule.forFeature([Games], 'games'),
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
