import { Module } from '@nestjs/common';
import { TournamentsController } from './controllers/tournaments.controller';
import { ThrottlerModule } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import { TournamentsService } from './services/tournaments.service';
import { Tournament } from './entities/tournament.entity';
import { Game } from './entities/game.entity';
import { GameRules } from './entities/gameRules.entity';
import { Matchup } from './entities/matchup.entity';
import { Group } from './entities/group.entity';
import { Stage } from './entities/stage.entity';
import { GameService } from './services/game.service';
import { GameController } from './controllers/game.controller';
import { SeeksService } from './services/seeks.service';
import { SeeksController } from './controllers/seeks.controller';
import { HttpModule } from '@nestjs/axios';
import { MatchupsController } from './controllers/matchups.controller';
import { MatchupsService } from './services/matchups.service';
import { GameRulesService } from './services/gameRules.service';
import { GameRulesController } from './controllers/gameRules.controller';
import { ConfigService } from '@nestjs/config';
import { StagesController } from './controllers/stages.controller';
import { GroupsController } from './controllers/groups.controller';
import { StagesService } from './services/stages.service';
import { GroupsService } from './services/groups.service';

@Module({
	controllers: [
		SeeksController,
		GameRulesController,
		TournamentsController,
		StagesController,
		GroupsController,
		MatchupsController,
		GameController,
	],
	imports: [
		HttpModule,
		TypeOrmModule.forFeature([
			Tournament, Stage, Group, Matchup, Game, GameRules]),
		ThrottlerModule.forRootAsync({
			useFactory: () => ({
				ttl: 60,
				limit: 60,
			}),
		}),
	],
	providers: [
		SeeksService,
		GameRulesService,
		TournamentsService,
		StagesService,
		GroupsService,
		MatchupsService,
		GameService,
		ConfigService,
	],
})
export class TournamentsModule { }
