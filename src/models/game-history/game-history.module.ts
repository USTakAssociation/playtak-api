import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Games } from './entities/games.entity';
import { GameHistoryController } from './game-history.controller';
import { GameHistoryService } from './game-history.service';

@Module({
	controllers: [GameHistoryController],
	imports: [TypeOrmModule.forFeature([Games])],
	providers: [GameHistoryService],
})
export class GameHistoryModule {}
