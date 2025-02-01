/* istanbul ignore file */
import { Module } from '@nestjs/common';
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
		TypeOrmModule.forFeature([Ratings, Players, Games]),
		ThrottlerModule.forRootAsync({
			useFactory: () => [{
				ttl: 60,
				limit: 60,
			}],
		}),
	],
	providers: [RatingTask, RatingService],
})
export class RatingsModule {}
