/* istanbul ignore file */
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Games } from './entities/games.entity';
import { GamesController } from './games.controller';
import { GamesService } from './games.service';
import { PTNService } from './services/ptn.service';

@Module({
	controllers: [GamesController],
	imports: [TypeOrmModule.forFeature([Games], 'games')],
	providers: [GamesService, PTNService]
})
export class GamesModule {}
