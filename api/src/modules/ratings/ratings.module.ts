/* istanbul ignore file */
import { Module } from '@nestjs/common';
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
					ttl: 60,
					limit: 60
				}
			]
		})
	],
	providers: [RatingTask, RatingService]
})
export class RatingsModule {}
