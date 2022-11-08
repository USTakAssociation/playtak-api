import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { ThrottlerGuard, ThrottlerModule } from '@nestjs/throttler';
import { APP_GUARD } from '@nestjs/core';
import { ScheduleModule } from '@nestjs/schedule';
import { EventsModule } from './models/events/events.module';

@Module({
	imports: [
		ConfigModule.forRoot(),
		ThrottlerModule.forRootAsync({
			useFactory: () => ({
				ttl: 60,
				limit: 5,
			}),
		}),
		ScheduleModule.forRoot(),
		EventsModule,
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
