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
		// TODO: ThrottlerModule is @Global(), and forRootAsync is called separately
		// here and in events.module.ts/ratings.module.ts with different limits.
		// Verify these don't clobber each other (last-imported module winning globally).
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
