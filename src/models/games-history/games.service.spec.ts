import { Test, TestingModule } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Games } from './entities/games.entity';
import { GamesService } from './games.service';
import { PTNService } from './services/ptn.service';

describe('GamesService', () => {
	let service: GamesService;

	const mockRepo = {
		findAndCount: jest.fn().mockImplementation(() => [[{ sn: '1234' }], 1]),
		findOne: jest.fn(),
		findByIds: jest.fn(),
		find: jest.fn(),
		save: jest.fn(),
		update: jest.fn(),
		delete: jest.fn(),
		createQueryBuilder: jest.fn(() => ({
			select: () => jest.fn(),
			where: () => jest.fn(),
			orWhere: () => jest.fn(),
			from: () => jest.fn(),
			whereInIds: () => jest.fn(),
			orderBy:() => jest.fn(),
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
			providers: [
				GamesService,
				PTNService,
				{
					provide: getRepositoryToken(Games),
					useValue: mockRepo,
				},
			],
		}).compile();

		service = module.get<GamesService>(GamesService);
	});

	it('should be defined', () => {
		expect(service).toBeDefined();
	});
	
	describe('Generate Search Query', () => {
		it('Should return the correct values for player white and empty for mirror', () => {
			const mockQuery = { player_white: 'bcreature', mirror: 'false'};
			const {search, mirrorSearch} = service.generateSearchQuery(mockQuery);
			expect(search['player_white']._value).toEqual('bcreature');
			expect(search['player_white']._type).toEqual('like');
			expect(search['date']._value).toEqual('1461430800000');
			expect(search['date']._type).toEqual('moreThan');
			expect(mirrorSearch).toStrictEqual({});
		});
		
		it('Should return the correct values for player black and empty for mirror', () => {
			const mockQuery = {
				player_black: 'bcreature',
				mirror: 'true',
			};
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['player_black']._value).toEqual('bcreature');
			expect(search['player_black']._type).toEqual('like');
			expect(search['date']._value).toEqual('1461430800000');
			expect(search['date']._type).toEqual('moreThan');
			expect(mirrorSearch['player_white']._value).toEqual('bcreature');
			expect(mirrorSearch['player_white']._type).toEqual('like');
		});
		
		it('Should return the correct values for player white and empty for mirror', () => {
			const mockQuery = {
				player_white: 'bcreature',
				mirror: 'true',
			};
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['player_white']._value).toEqual('bcreature');
			expect(search['player_white']._type).toEqual('like');
			expect(search['date']._value).toEqual('1461430800000');
			expect(search['date']._type).toEqual('moreThan');
			expect(mirrorSearch['player_black']._value).toEqual('bcreature');
			expect(mirrorSearch['player_black']._type).toEqual('like');
		});
		
		it('Should return the correct values for normal', () => {
			const mockQuery = {
				type: "normal",
				mirror: 'false'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['tournament']).toBe(0);
			expect(search['unrated']).toBe(0);
			expect(search['normal']).toBe(1);
			expect(mirrorSearch).toStrictEqual({});
		});

		it('Should return the correct game result X-0', () => {
			const mockQuery = {
				game_result: 'X-0',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } =
				service.generateSearchQuery(mockQuery);
			expect(search['result']._value).toEqual('%-0');
			expect(search['result']._type).toEqual('like');
			expect(mirrorSearch['result']._value).toEqual('0-%');
			expect(mirrorSearch['result']._type).toEqual('like');
		});

		it('Should return the correct game result 0-X', () => {
			const mockQuery = {
				game_result: '0-X',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } =
				service.generateSearchQuery(mockQuery);
			expect(search['result']._value).toEqual('0-%');
			expect(search['result']._type).toEqual('like');
			expect(mirrorSearch['result']._value).toEqual('%-0');
			expect(mirrorSearch['result']._type).toEqual('like');
		});
		
		it('Should return the correct game result 0-F', () => {
			const mock = { game_result: '0-F', mirror: 'true' } as const;
			const { search, mirrorSearch } =
				service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-F');
			expect(mirrorSearch['result']).toEqual('F-0');
		});
		
		it('Should return the correct game result F-0', () => {
			const mock = {
				game_result: 'F-0',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } =
				service.generateSearchQuery(mock);
			expect(search['result']).toEqual('F-0');
			expect(mirrorSearch['result']).toEqual('0-F');
		});
		it('Should return the correct game result 1/2-1/2', () => {
			const mock = {
				game_result: '1/2-1/2',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } =
				service.generateSearchQuery(mock);
			expect(search['result']).toEqual('1/2-1/2');
			expect(mirrorSearch['result']).toEqual('1/2-1/2');
		});
		it('Should return the correct game result R-0', () => {
			const mock = {
				game_result: 'R-0',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('R-0');
			expect(mirrorSearch['result']).toEqual('0-R');
		});
		it('Should return the correct game result 0-R', () => {
			const mock = {
				game_result: '0-R',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-R');
			expect(mirrorSearch['result']).toEqual('R-0');
		});
		it('Should return the correct game result 0-F', () => {
			const mock = {
				game_result: '0-F',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-F');
			expect(mirrorSearch['result']).toEqual('F-0');
		});
		it('Should return the correct game result 0-1', () => {
			const mock = {
				game_result: '0-1',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-1');
			expect(mirrorSearch['result']).toEqual('1-0');
		});
		it('Should return the correct game result 1-0', () => {
			const mock = {
				game_result: '1-0',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('1-0');
			expect(mirrorSearch['result']).toEqual('0-1');
		});
		it('Should return the correct game result 3-0', () => {
			const mock = {
				game_result: '3-0',
				mirror: 'true',
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('3-0');
			expect(mirrorSearch['result']).toEqual('3-0');
		});
		it('Should return the correct search for ID', () => {
			const mock = {
				id: 1234,
				mirror: 'true',
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['id']).toEqual(1234);
		});
		it('Should return the correct search for size', () => {
			const mock = {
				size: 7,
				mirror: 'true',
			} as const;
			const { search } =
				service.generateSearchQuery(mock);
			expect(search['size']).toEqual(7);
		});
	})
});
