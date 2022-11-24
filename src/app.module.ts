import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { ThrottlerGuard } from '@nestjs/throttler';
import { APP_GUARD } from '@nestjs/core';
import { ScheduleModule } from '@nestjs/schedule';
import { EventsModule } from './models/events/events.module';
import { GamesModule } from './models/games-history/games.module';
import { TypeOrmModule } from '@nestjs/typeorm';

@Module({
	imports: [
		ConfigModule.forRoot(),
		TypeOrmModule.forRoot({
			type: 'sqlite',
			database: process.env.DB_PATH + 'games.db',
			entities: [__dirname + '/**/*.entity{.ts,.js}'],
			synchronize: false,
		}),
		ScheduleModule.forRoot(),
		EventsModule,
		GamesModule,
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
