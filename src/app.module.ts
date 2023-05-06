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

@Module({
	imports: [
		ConfigModule.forRoot(),
		TypeOrmModule.forRoot({
			name: 'games',
			type: 'sqlite',
			database: path.join(process.env.DB_PATH, 'games.db'),
			entities: [__dirname + '/**/*.entity{.ts,.js}'],
			synchronize: false,
		}),
		TypeOrmModule.forRoot({
			name: 'default',
			type: 'sqlite',
			database:  path.join(process.env.DB_PATH, 'players.db'),
			entities: [__dirname + '/**/*.entity{.ts,.js}'],
			synchronize: false,
		}),
		ScheduleModule.forRoot(),
		EventsModule,
		GamesModule,
		RatingsModule,
	],
	controllers: [],
	providers: [
		{
			provide: APP_GUARD,
			useClass: ThrottlerGuard,
		},
	],
})
export class AppModule {}
