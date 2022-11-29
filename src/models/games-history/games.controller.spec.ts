import { Test, TestingModule } from '@nestjs/testing';
import { GamesController } from './games.controller';
import { GamesService } from './games.service';

describe('GamesController', () => {
	let controller: GamesController;

	const mockService = {
	};
	
	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			controllers: [GamesController],
			providers: [{
				provide: GamesService,
				useValue: mockService
			}]
		}).compile();

		controller = module.get<GamesController>(GamesController);
	});

	it('should be defined', () => {
		expect(controller).toBeDefined();
	});
});
