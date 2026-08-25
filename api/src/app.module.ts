/* istanbul ignore file */
import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { APP_GUARD } from '@nestjs/core';
import { ScheduleModule } from '@nestjs/schedule';
import { ThrottlerGuard } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import * as path from 'path';
import { AppController } from './app.controller';
import { EventsModule } from './modules/events/events.module';
import { GamesModule } from './modules/games-history/games.module';
import { RatingsModule } from './modules/ratings/ratings.module';

@Module({
	imports: [
		ConfigModule.forRoot(),
		TypeOrmModule.forRoot({
			name: 'default',
			type: 'better-sqlite3',
			database: path.join(process.env.DB_PATH, 'players.db'),
			entities: [path.join(__dirname, '/**/*.entity{.ts,.js}')],
			synchronize: false
		}),
		TypeOrmModule.forRoot({
			name: 'games',
			type: 'better-sqlite3',
			database: path.join(process.env.DB_PATH, 'games.db'),
			entities: [path.join(__dirname, '/**/*.entity{.ts,.js}')],
			synchronize: false
		}),
		ScheduleModule.forRoot(),
		EventsModule,
		GamesModule,
		RatingsModule
	],
	controllers: [AppController],
	providers: [
		{
			provide: APP_GUARD,
			useClass: ThrottlerGuard
		}
	]
})
export class AppModule {}
