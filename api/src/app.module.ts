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
// import * as path from 'path';
import { AppController } from './app.controller';

@Module({
	imports: [
		ConfigModule.forRoot({isGlobal: true}),
		TypeOrmModule.forRoot({
			type: process.env.DB_TYPE as 'mysql' || 'mysql',
			host: process.env.DB_HOST || 'localhost',
			port: parseInt(process.env.DB_PORT) || 3306,
			username: process.env.DB_USERNAME,
			password: process.env.DB_PASSWORD,
			database: process.env.DB_NAME,
			entities: [__dirname + '/**/*.entity{.ts,.js}'],
			synchronize: false,
			connectTimeout: 30000,
			extra: {
				idleTimeout: 30000,
				maxIdle: 0,
				enableKeepAlive: true,
				keepAliveInitialDelay: 300,
			}
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
