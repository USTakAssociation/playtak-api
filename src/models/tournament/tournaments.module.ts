import { Module } from '@nestjs/common';
import { TournamentsController } from './tournaments.controller';
import { ThrottlerModule } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import { TournamentsService } from './tournaments.service';
import { Tournament } from './entities/tournament.entity';

@Module({
	controllers: [TournamentsController],
	imports: [
		TypeOrmModule.forFeature([Tournament]),
		ThrottlerModule.forRootAsync({
			useFactory: () => ({
				ttl: 60,
				limit: 60,
			}),
		}),
	],
	providers: [TournamentsService],
})
export class TournamentsModule {}
