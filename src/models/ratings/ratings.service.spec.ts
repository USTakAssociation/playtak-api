import { Test, TestingModule } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Ratings } from './entities/ratings.entity';
import { RatingService } from './ratings.service';
import { Players } from './entities/players.entity';
import { Games } from '../games-history/entities/games.entity';

describe('RatingService', () => {
	let service: RatingService;

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
	const INITIAL_RATING = 1000;
	const BONUS_RATING = 750;
	const BONUS_FACTOR = 60;
	const PARTICIPATION_LIMIT = 10;
	const PARTICIPATION_CUTOFF = 1500;
	const MAX_DROP = 200;
	const RATING_RETENTION = 1000 * 60 * 60 * 24 * 240;
		
	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			providers: [
				RatingService,
				{
					provide: getRepositoryToken(Ratings, 'default'),
					useValue: mockRepo,
				},
				{
					provide: getRepositoryToken(Players, 'default'),
					useValue: mockRepo,
				},
				{
					provide: getRepositoryToken(Games, 'games'),
					useValue: mockRepo,
				},
			],
		}).compile();

		service = module.get<RatingService>(RatingService);
	});

	it('should be defined', () => {
		expect(service).toBeDefined();
	});
	
	describe('Adjust Player', () => {
		it('should return adjusted player', async () => {
			const mockGame = {
				result: 'R-0',
				date: 1683328933,
			}
			const quickresult = {'R-0': 1, 'F-0': 1, '1-0': 1, '0-R': 0, '0-F': 0, '0-1': 0, '1/2-1/2': 0.5}[mockGame.result];
			const player_white = {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 1637395624533.48
			}
			const player_black = {
				id: 2,
				fatigue: {
					"1": 0.10901345963072347
				},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 0
			}

			const sw = Math.pow(10, player_white.rating / 400);
			const sb = Math.pow(10, player_black.rating / 400);
			const expected = sw / (sw + sb);
			const fairness = expected * (1 - expected);
			const fatiguefactor =
				(1 -(player_white.fatigue[player_black.id] || 0) * 0.4) *
				(1 -(player_black.fatigue[player_white.id] || 0) * 0.4);
			
			const white_result = await service.adjustPlayer(player_white, quickresult - expected, fairness, fatiguefactor, mockGame.date, BONUS_FACTOR, BONUS_RATING, RATING_RETENTION, INITIAL_RATING);
			const black_result = await service.adjustPlayer(player_black, quickresult - expected, fairness, fatiguefactor, mockGame.date, BONUS_FACTOR, BONUS_RATING, RATING_RETENTION, INITIAL_RATING);
			const mock_white_result =  {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 1,
				rating: 1019.1278923229543,
				maxrating: 1019.1278923229543,
				ratingage: 1683328933,
				changed: true
			}
			const mock_black_result = {
				id: 2,
				fatigue: { '1': 0.10901345963072347 },
				boost: 0,
				ratedgames: 1,
				rating: 1019.1278923229543,
				maxrating: 1019.1278923229543,
				ratingage: -18345807353.537495,
				changed: true
			}
			expect(mock_white_result).toMatchObject(white_result);
			expect(mock_black_result).toMatchObject(black_result);
		});
	})

	describe('Adjust Fatigue', () => {
		it('should return adjusted fatigue', async () => {
			const player_white = {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 1637395624533.48
			}
			const player_black = {
				id: 2,
				fatigue: {
					"1": 0.10901345963072347
				},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 0
			}
			const sw = Math.pow(10, player_white.rating / 400);
			const sb = Math.pow(10, player_black.rating / 400);
			const expected = sw / (sw + sb);
			const fairness = expected * (1 - expected);
			const fatiguefactor =
				(1 -(player_white.fatigue[player_black.id] || 0) * 0.4) *
				(1 -(player_black.fatigue[player_white.id] || 0) * 0.4);
			const white_result = await service.updateFatigue(player_white, player_black.id.toString(), fairness * fatiguefactor);
			const black_result = await service.updateFatigue(player_black, player_white.id.toString(), fairness * fatiguefactor);
			expect(white_result).toEqual(0.23909865403692765);
			expect(black_result).toEqual(0.3376861250798051);
		});
		
		it('should return an error if fatigue is a string', async () => {
			const player_white = {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 1637395624533.48
			}
			const player_black = {
				id: 2,
				fatigue: '{ "1": 0.10901345963072347}',
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 0
			}
			const sw = Math.pow(10, player_white.rating / 400);
			const sb = Math.pow(10, player_black.rating / 400);
			const expected = sw / (sw + sb);
			const fairness = expected * (1 - expected);
			const fatiguefactor =
				(1 -(player_white.fatigue[player_black.id] || 0) * 0.4) *
				(1 -(0) * 0.4);
			try {
				await service.updateFatigue(player_black, player_white.id.toString(), fairness * fatiguefactor);
			} catch (error) {
				expect(error.message).toEqual("Fatigue is a string, needs to be an object");
			}
		});
		
		it('should delete the player fatigue', async () => {
			const player_white = {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 1637395624533.48
			}
			// fatigue opponent is 0 and less that 0.01
			const player_black = {
				id: 2,
				fatigue: { "0": 0.00901345963072347},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 0
			}
			const sw = Math.pow(10, player_white.rating / 400);
			const sb = Math.pow(10, player_black.rating / 400);
			const expected = sw / (sw + sb);
			const fairness = expected * (1 - expected);
			const fatiguefactor =
				(1 -(player_white.fatigue[player_black.id] || 0) * 0.4) *
				(1 -(0) * 0.4);
			const result = await service.updateFatigue(player_black, player_white.id.toString(), fairness * fatiguefactor);
			expect(result).toEqual(0.25);
		});
		
		
	});
	
	describe('Adjust Rating', () => {
		it('should return adjusted rating', async () => {
			const mockGame = {
				result: 'R-0',
				date: 1683328933,
			}

			const player_white = {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 0,
				rating: 1600,
				maxrating: 1000,
				ratingage: 1637395624533.48
			}
			const player_black = {
				id: 2,
				fatigue: {
					"1": 0.10901345963072347
				},
				boost: 0,
				ratedgames: 0,
				rating: 1000,
				maxrating: 1000,
				ratingage: 0
			}
			
			const white_result = await service.adjustedRating(player_white, mockGame.date, PARTICIPATION_CUTOFF, RATING_RETENTION, MAX_DROP, PARTICIPATION_LIMIT);
			const black_result = await service.adjustedRating(player_black, mockGame.date, PARTICIPATION_CUTOFF, RATING_RETENTION, MAX_DROP, PARTICIPATION_LIMIT);
			expect(white_result).toEqual(1600);
			expect(black_result).toEqual(1000);
		});

		it('should return adjusted rating past the participation + max drop', async () => {
			const mockGame = {
				result: 'R-0',
				date: 1683328933,
			}
			const player_white = {
				id: 1,
				fatigue: {},
				boost: 0,
				ratedgames: 0,
				rating: 1700,
				maxrating: 1000,
				ratingage: 1637395624533.48
			}
			const white_result = await service.adjustedRating(player_white, mockGame.date, PARTICIPATION_CUTOFF, RATING_RETENTION, MAX_DROP, PARTICIPATION_LIMIT);
			expect(white_result).toEqual(1700);
		});
	})
	
	describe('Is Game Eligible', () => {
		it('should return true', async () => {
			const game1 = {size: 4, pieces: 30, capstones: 1, timertime: 61, timerinc: 10};
			const game2 = {size: 5, pieces: 30, capstones: 1, timertime: 61, timerinc: 10};
			const r1 = await service.isGameEligible(game1);
			const r2 = await service.isGameEligible(game2);
			expect(r1).toEqual(false);
			expect(r2).toEqual(true);
		});

	})
});
