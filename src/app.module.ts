import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { ThrottlerGuard } from '@nestjs/throttler';
import { APP_GUARD } from '@nestjs/core';
import { EventsModule } from './models/events/events.module';
import { GamesModule } from './models/games-history/games.module';
import { TypeOrmModule } from '@nestjs/typeorm';
import { TournamentsModule } from './models/tournament/tournaments.module';
import * as path from 'path';


@Module({
	imports: [
		ConfigModule.forRoot(),
		TypeOrmModule.forRoot({
			type: 'sqlite',
			database: path.join(process.env.DB_PATH, 'games.db'),
			entities: [path.join(__dirname, '/**/*.entity{.ts,.js}')],
			// Only re-create the DB Schemas in dev mode. For production this must be disabled.
			synchronize: process.env['NODE_ENV'] == 'local',
		}),
		EventsModule,
		GamesModule,
		TournamentsModule,
	],
	controllers: [],
	providers: [
		{
			provide: APP_GUARD,
			useClass: ThrottlerGuard,
		}
	],
})
export class AppModule {}
