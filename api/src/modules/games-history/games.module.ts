/* istanbul ignore file */
import { Module } from '@nestjs/common';
import { CacheModule } from '@nestjs/cache-manager';
import { ThrottlerModule } from '@nestjs/throttler';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Games } from './entities/games.entity';
import { GamesController } from './games.controller';
import { GamesService } from './games.service';
import { PTNService } from './services/ptn.service';

@Module({
	controllers: [GamesController],
	imports: [
		TypeOrmModule.forFeature([Games], 'games'),
		ThrottlerModule.forRootAsync({
			useFactory: () => [
				{
					ttl: 60000,
					limit: 20
				}
			]
		}),
		CacheModule.register({
			ttl: 300, // seconds
			max: 200 // maximum number of items in cache
		})
	],
	providers: [GamesService, PTNService]
})
export class GamesModule {}
