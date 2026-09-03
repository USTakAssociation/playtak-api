import { Test, TestingModule } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { vi } from 'vitest';
import { Games } from './entities/games.entity';
import { GamesService } from './games.service';
import { PTNService } from './services/ptn.service';

describe('GamesService', () => {
	let service: GamesService;

	const mockRepo = {
		findAndCount: vi.fn().mockImplementation(() => [[{ sn: '1234' }], 1]),
		findOne: vi.fn(),
		findByIds: vi.fn(),
		find: vi.fn(),
		save: vi.fn(),
		update: vi.fn(),
		delete: vi.fn(),
		createQueryBuilder: vi.fn(() => ({
			select: () => vi.fn(),
			where: () => vi.fn(),
			orWhere: () => vi.fn(),
			from: () => vi.fn(),
			whereInIds: () => vi.fn(),
			orderBy: () => vi.fn(),
			groupBy: () => vi.fn(),
			delete: () => vi.fn(),
			execute: () => vi.fn()
		})),
		manager: {
			connection: {
				transaction: vi.fn()
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

	describe('Sanitizes query params', () => {
		it('Should drop an id that fails validation instead of searching on NaN', () => {
			const { search } = service.generateSearchQuery({ id: 'abc1', mirror: 'false' });
			expect(search).not.toHaveProperty('id');
		});

		it('Should drop a reversed id range rather than silently reversing it', () => {
			const { search } = service.generateSearchQuery({ id: '10-1', mirror: 'false' });
			expect(search).not.toHaveProperty('id');
		});

		it.each([['>abc'], ['<abc'], ['abc'], ['abc-def'], ['1622505600000-def']])(
			'Should drop the date filter for %s',
			(date) => {
				const { search } = service.generateSearchQuery({ date, mirror: 'false' });
				expect(search).not.toHaveProperty('date');
			}
		);

		it('Should still order a reversed date range', () => {
			const { search } = service.generateSearchQuery({
				date: '1625097600000-1622505600000',
				mirror: 'false'
			});
			expect(search['date']._value).toEqual([1622505600000, 1625097600000]);
			expect(search['date']._type).toEqual('between');
		});

		it.each([
			['size'],
			['timertime'],
			['timerinc'],
			['extra_time_amount'],
			['extra_time_trigger'],
			['increment_scales']
		])('Should drop a non-numeric %s', (field) => {
			const { search } = service.generateSearchQuery({ [field]: '7; DROP TABLE games', mirror: 'false' });
			expect(search).not.toHaveProperty(field);
		});

		it('Should accept increment_scales as a number', () => {
			const { search } = service.generateSearchQuery({ increment_scales: '1', mirror: 'false' });
			expect(search['increment_scales']).toEqual(1);
		});

		it.each([['tournament'], ['unrated']])('Should accept the %s type filter', (type) => {
			const { search } = service.generateSearchQuery({ type, mirror: 'false' });
			expect(search[type]).toEqual(1);
		});

		it('Should accept a type filter in any case', () => {
			const { search } = service.generateSearchQuery({ type: 'TOURNAMENT', mirror: 'false' });
			expect(search['tournament']).toEqual(1);
		});

		it('Should ignore a type filter that is not an allowed column', () => {
			const { search } = service.generateSearchQuery({ type: 'notation', mirror: 'false' });
			expect(search).toStrictEqual({});
		});
	});

	describe('getAll bounds', () => {
		let captured: { sort?: string; order?: string; limit?: number; offset?: number };

		beforeEach(() => {
			captured = {};
			const builder = {
				select: () => builder,
				where: () => builder,
				orWhere: () => builder,
				clone: () => builder,
				orderBy: (sort: string, order: string) => {
					captured.sort = sort;
					captured.order = order;
					return builder;
				},
				limit: (value: number) => {
					captured.limit = value;
					return builder;
				},
				offset: (value: number) => {
					captured.offset = value;
					return builder;
				},
				getCount: async () => 0,
				execute: async () => []
			};
			mockRepo.createQueryBuilder.mockReturnValue(builder);
		});

		it('Should default to 50 rows of the newest ids', async () => {
			await service.getAll({ mirror: 'false' } as never);
			expect(captured).toEqual({ sort: 'id', order: 'DESC', limit: 50, offset: 0 });
		});

		it.each([
			['1000', 200],
			['0', 1],
			['-10', 1],
			['abc', 50],
			['25', 25]
		])('Should clamp limit=%s to %s', async (limit, expected) => {
			await service.getAll({ limit, mirror: 'false' } as never);
			expect(captured.limit).toEqual(expected);
		});

		it('Should turn page and limit into an offset', async () => {
			await service.getAll({ limit: '10', page: '3', mirror: 'false' } as never);
			expect(captured.offset).toEqual(30);
		});

		it('Should fall back to skip when there is no page', async () => {
			await service.getAll({ skip: '25', mirror: 'false' } as never);
			expect(captured.offset).toEqual(25);
		});

		it.each([['-5'], ['abc']])('Should floor page=%s at zero', async (page) => {
			await service.getAll({ page, mirror: 'false' } as never);
			expect(captured.offset).toEqual(0);
		});

		it('Should sort by an allowed column', async () => {
			await service.getAll({ sort: 'date', mirror: 'false' } as never);
			expect(captured.sort).toEqual('date');
		});

		it.each([['notation; DROP TABLE games'], ['(SELECT 1)'], ['unknown_column']])(
			'Should fall back to id for sort=%s',
			async (sort) => {
				await service.getAll({ sort, mirror: 'false' } as never);
				expect(captured.sort).toEqual('id');
			}
		);

		it.each([
			['ASC', 'ASC'],
			['asc', 'ASC'],
			['DESC', 'DESC'],
			['sideways', 'DESC']
		])('Should resolve order=%s to %s', async (order, expected) => {
			await service.getAll({ order, mirror: 'false' } as never);
			expect(captured.order).toEqual(expected);
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
