import { Test, TestingModule } from '@nestjs/testing';
import { GameHistoryService } from './game-history.service';

describe('GameHistoryService', () => {
	let service: GameHistoryService;

	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			providers: [GameHistoryService],
		}).compile();

		service = module.get<GameHistoryService>(GameHistoryService);
	});

	it('should be defined', () => {
		expect(service).toBeDefined();
	});
});
