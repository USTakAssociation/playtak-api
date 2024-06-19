import { Test, TestingModule } from '@nestjs/testing';
import { RatingsController } from './ratings.controller';
import { RatingService } from './ratings.service';

describe('RatingsController', () => {
	let controller: RatingsController;

	const mockService = {
	};
	
	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			controllers: [RatingsController],
			providers: [{
				provide: RatingService,
				useValue: mockService
			}]
		}).compile();

		controller = module.get<RatingsController>(RatingsController);
	});

	it('should be defined', () => {
		expect(controller).toBeDefined();
	});
});
