import { Module } from '@nestjs/common';
import { TournamentsController } from './tournaments.controller';
import { ThrottlerModule } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import { TournamentsService } from './tournaments.service';
import { Tournament } from './entities/tournament.entity';
import { Game } from './entities/game.entity';
import { GameRules } from './entities/gameRules.entity';
import { Matchup } from './entities/matchup.entity';
import { Group } from './entities/group.entity';
import { Stage } from './entities/stage.entity';
import { GameService } from './services/game.service';
import { GameController } from './game.controller';

@Module({
	controllers: [TournamentsController, GameController],
	imports: [
		TypeOrmModule.forFeature([Tournament, Stage, Group, Matchup, Game, GameRules]),
		ThrottlerModule.forRootAsync({
			useFactory: () => ({
				ttl: 60,
				limit: 60,
			}),
		}),
	],
	providers: [TournamentsService, GameService],
})
export class TournamentsModule {}
