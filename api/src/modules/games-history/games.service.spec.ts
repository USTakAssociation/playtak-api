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
			orderBy: () => jest.fn(),
			groupBy: () => jest.fn(),
			delete: () => jest.fn(),
			execute: () => jest.fn()
		})),
		manager: {
			connection: {
				transaction: jest.fn()
			}
		}
	};

	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			providers: [
				GamesService,
				PTNService,
				{
					provide: getRepositoryToken(Games, 'games'),
					useValue: mockRepo
				}
			]
		}).compile();

		service = module.get<GamesService>(GamesService);
	});

	it('should be defined', () => {
		expect(service).toBeDefined();
	});

	describe('Generate Search Query', () => {
		// handle single game id search
		it('Should return the correct values for single game id search', () => {
			const mockQuery = { id: '1234', mirror: 'false' };
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['id']).toBe(1234);
			expect(mirrorSearch).toStrictEqual({});
		});
		// handle game id range search
		it('Should return the correct values for game id range search', () => {
			const mockQuery = { id: '10-50', mirror: 'false' };
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['id']._value).toEqual([10, 50]);
			expect(search['id']._type).toEqual('between');
			expect(mirrorSearch).toStrictEqual({});
		});
		// handle game id comma separated search
		it('Should return the correct values for game id comma separated search', () => {
			const mockQuery = { id: '10,20,30', mirror: 'false' };
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['id']._value).toEqual([10, 20, 30]);
			expect(search['id']._type).toEqual('in');
			expect(mirrorSearch).toStrictEqual({});
		});
		it('Should return the correct values for player white and empty for mirror', () => {
			const mockQuery = { player_white: 'bcreature', mirror: 'false' };
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['player_white']._value).toEqual('bcreature');
			expect(search['player_white']._type).toEqual('like');
			expect(mirrorSearch).toStrictEqual({});
		});

		it('Should return the correct values for player black and empty for mirror', () => {
			const mockQuery = {
				player_black: 'bcreature',
				mirror: 'true'
			};
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['player_black']._value).toEqual('bcreature');
			expect(search['player_black']._type).toEqual('like');
			expect(mirrorSearch['player_white']._value).toEqual('bcreature');
			expect(mirrorSearch['player_white']._type).toEqual('like');
		});

		it('Should return the correct values for player white and empty for mirror', () => {
			const mockQuery = {
				player_white: 'bcreature',
				mirror: 'true'
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
				type: 'normal',
				mirror: 'false'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['tournament']).toBe(0);
			expect(search['unrated']).toBe(0);
			expect(mirrorSearch).toStrictEqual({});
		});

		it('Should return the correct game result X-0', () => {
			const mockQuery = {
				game_result: 'X-0',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['result']._value).toEqual('%-0');
			expect(search['result']._type).toEqual('like');
			expect(mirrorSearch['result']._value).toEqual('0-%');
			expect(mirrorSearch['result']._type).toEqual('like');
		});

		it('Should return the correct game result 0-X', () => {
			const mockQuery = {
				game_result: '0-X',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mockQuery);
			expect(search['result']._value).toEqual('0-%');
			expect(search['result']._type).toEqual('like');
			expect(mirrorSearch['result']._value).toEqual('%-0');
			expect(mirrorSearch['result']._type).toEqual('like');
		});

		it('Should return the correct game result 0-F', () => {
			const mock = { game_result: '0-F', mirror: 'true' } as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-F');
			expect(mirrorSearch['result']).toEqual('F-0');
		});

		it('Should return the correct game result F-0', () => {
			const mock = {
				game_result: 'F-0',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('F-0');
			expect(mirrorSearch['result']).toEqual('0-F');
		});
		it('Should return the correct game result 1/2-1/2', () => {
			const mock = {
				game_result: '1/2-1/2',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('1/2-1/2');
			expect(mirrorSearch['result']).toEqual('1/2-1/2');
		});
		it('Should return the correct game result R-0', () => {
			const mock = {
				game_result: 'R-0',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('R-0');
			expect(mirrorSearch['result']).toEqual('0-R');
		});
		it('Should return the correct game result 0-R', () => {
			const mock = {
				game_result: '0-R',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-R');
			expect(mirrorSearch['result']).toEqual('R-0');
		});
		it('Should return the correct game result 0-F', () => {
			const mock = {
				game_result: '0-F',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-F');
			expect(mirrorSearch['result']).toEqual('F-0');
		});
		it('Should return the correct game result 0-1', () => {
			const mock = {
				game_result: '0-1',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('0-1');
			expect(mirrorSearch['result']).toEqual('1-0');
		});
		it('Should return the correct game result 1-0', () => {
			const mock = {
				game_result: '1-0',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('1-0');
			expect(mirrorSearch['result']).toEqual('0-1');
		});
		it('Should return the correct game result 3-0', () => {
			const mock = {
				game_result: '3-0',
				mirror: 'true'
			} as const;
			const { search, mirrorSearch } = service.generateSearchQuery(mock);
			expect(search['result']).toEqual('3-0');
			expect(mirrorSearch['result']).toEqual('3-0');
		});
		it('Should return the correct search for ID', () => {
			const mock = {
				id: '1234',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['id']).toEqual(1234);
		});
		it('Should return the correct search for size', () => {
			const mock = {
				size: 7,
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['size']).toEqual(7);
		});

		// handle single date search
		it('Should return the correct search for single date', () => {
			const mock = {
				date: '1622505600000',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['date']).toBe(1622505600000);
		});
		// handle date range search
		it('Should return the correct search for date range', () => {
			const mock = {
				date: '1622505600000-1625097600000',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['date']._value).toEqual([1622505600000, 1625097600000]);
			expect(search['date']._type).toEqual('between');
		});
		// handle greater than date search
		it('Should return the correct search for greater than date', () => {
			const mock = {
				date: '>1622505600000',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['date']._value).toEqual(1622505600000);
			expect(search['date']._type).toEqual('moreThan');
		});
		// handle less than date search
		it('Should return the correct search for less than date', () => {
			const mock = {
				date: '<1625097600000',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['date']._value).toEqual(1625097600000);
			expect(search['date']._type).toEqual('lessThan');
		});

		// should return the correct search for timertime
		it('Should return the correct search for timertime', () => {
			const mock = {
				timertime: '300',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['timertime']).toEqual(300);
		});

		// should return the correct search for timerinc
		it('Should return the correct search for timerinc', () => {
			const mock = {
				timerinc: '10',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['timerinc']).toEqual(10);
		});

		// should return the correct search for extra_time_amount
		it('Should return the correct search for extra_time_amount', () => {
			const mock = {
				extra_time_amount: '60',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['extra_time_amount']).toEqual(60);
		});

		// should return the correct search for extra_time_trigger
		it('Should return the correct search for extra_time_trigger', () => {
			const mock = {
				extra_time_trigger: '20',
				mirror: 'true'
			} as const;
			const { search } = service.generateSearchQuery(mock);
			expect(search['extra_time_trigger']).toEqual(20);
		});
	});

	describe('validate ID search', () => {
		it('Should return true for valid ID search 1234', () => {
			const idString = '1234';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(true);
		});
		it('Should return true for valid ID search', () => {
			const idString = '1-10';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(true);
		});
		it('Should return true for valid ID search 1,2,3,4', () => {
			const idString = '1,2,3,4';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(true);
		});
		it('Should return false for invalid ID abc1', () => {
			const idString = 'abc1';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
		it('Should return false for invalid ID 10-', () => {
			const idString = '10-';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
		it('Should return false for invalid ID 10-1', () => {
			const idString = '10-1';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
		it('Should return false for invalid ID 10-11,', () => {
			const idString = '10-11,';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
		it('Should return false for invalid ID 10-11-', () => {
			const idString = '10-11-';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
		it('Should return false for invalid ID 10---11', () => {
			const idString = '10--11';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
		it('Should return false for invalid ID 10,,11,1', () => {
			const idString = '10,,11,1';
			const result = service.validateIdQuery(idString);
			expect(result).toBe(false);
		});
	});
});
