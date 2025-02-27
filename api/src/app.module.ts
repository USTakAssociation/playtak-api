/* istanbul ignore file */
import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { ThrottlerGuard } from '@nestjs/throttler';
import { APP_GUARD } from '@nestjs/core';
import { EventsModule } from './models/events/events.module';
import { GamesModule } from './models/games-history/games.module';
import { TypeOrmModule } from '@nestjs/typeorm';
import { RatingsModule } from './models/ratings/ratings.module';
import { ScheduleModule } from '@nestjs/schedule';
import * as path from 'path';
import { AppController } from './app.controller';

@Module({
	imports: [
		ConfigModule.forRoot(),
		TypeOrmModule.forRoot({
			name: 'default',
			type: 'better-sqlite3',
			database: path.join(process.env.DB_PATH, 'players.db'),
			entities: [path.join(__dirname, '/**/*.entity{.ts,.js}')],
			synchronize: false,
		}),
		TypeOrmModule.forRoot({
			name: 'games',
			type: 'better-sqlite3',
			database: path.join(process.env.DB_PATH, 'games.db'),
			entities: [path.join(__dirname, '/**/*.entity{.ts,.js}')],
			synchronize: false,
		}),
		ScheduleModule.forRoot(),
		EventsModule,
		GamesModule,
		RatingsModule,
	],
	controllers: [AppController],
	providers: [
		{
			provide: APP_GUARD,
			useClass: ThrottlerGuard,
		},
	],
})
export class AppModule {}
