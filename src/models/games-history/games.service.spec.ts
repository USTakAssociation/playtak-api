import { Test, TestingModule } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Games } from './entities/games.entity';
import { GamesService } from './games.service';

describe('GamesService', () => {
	let service: GamesService;

	const mockRepo = {
		findAndCount: jest
			.fn()
			.mockImplementation(() => [[{ sn: '1234' }], 1]),
		findOne: jest.fn(),
		findByIds: jest.fn(),
		find: jest.fn(),
		save: jest.fn(),
		update: jest.fn(),
		delete: jest.fn(),
		createQueryBuilder: jest.fn(() => ({
			from: () => jest.fn(),
			whereInIds: () => jest.fn(),
			groupBy: () => jest.fn(),
			delete: () => jest.fn(),
			execute: () => jest.fn(),
		})),
		manager: {
			connection: {
				transaction: jest.fn(),
			},
		},
	};
		
	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			providers: [GamesService, {
				provide: getRepositoryToken(Games),
				useValue: mockRepo
			}],
		}).compile();

		service = module.get<GamesService>(GamesService);
	});

	it('should be defined', () => {
		expect(service).toBeDefined();
	});
});
