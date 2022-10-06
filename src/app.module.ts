import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { RatingModule } from './models/rating/rating.module';
import { GameHistoryModule } from './models/game-history/game-history.module';
import { UserModule } from './models/user/user.module';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ThrottlerGuard, ThrottlerModule } from '@nestjs/throttler';
import { APP_GUARD } from '@nestjs/core';
import { ScheduleModule } from '@nestjs/schedule';
@Module({
	imports: [
		ConfigModule.forRoot(),
		ThrottlerModule.forRootAsync({
			useFactory: () => ({
				ttl: 60,
				limit: 5,
			}),
		}),
		TypeOrmModule.forRootAsync({
			useFactory: () => ({
				name: 'games',
				type: 'better-sqlite3',
				database: process.env.DB_PATH + 'games.db',
				entities: [__dirname + '/**/*.entity{.ts,.js}'],
			}),
		}),
		TypeOrmModule.forRootAsync({
			useFactory: () => ({
				name: 'players',
				type: 'better-sqlite3',
				database: process.env.DB_PATH + 'players.db',
				entities: [__dirname + '/**/*.entity{.ts,.js}'],
			}),
		}),
		ScheduleModule.forRoot(),
		RatingModule,
		GameHistoryModule,
		UserModule,
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
