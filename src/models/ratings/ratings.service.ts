import { HttpException, Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Like, MoreThan, Repository } from 'typeorm';
import { Ratings } from './entities/ratings.entity';
import { RatingQuery } from './dto/ratings.dto';
import { access, readFile, writeFile } from 'fs/promises';
import { Players } from './entities/players.entity';
import { Games } from '../games-history/entities/games.entity';

@Injectable()
export class RatingService {
	private readonly logger = new Logger(RatingService.name);

	constructor(
		@InjectRepository(Ratings, 'default')
		private ratingRespository: Repository<Ratings>,
		@InjectRepository(Players, 'default')
		private playersRepository: Repository<Players>,
		@InjectRepository(Games, 'games')
		private gamesRepository: Repository<Games>,
	) {}

	async getAll(query?: RatingQuery): Promise<any> {
		const limit = parseInt(query.limit) || 50;
		const skip = parseInt(query.skip) || 0;
		const page = parseInt(query.page) || 0;
		const order: 'ASC' | 'DESC' = query.order || 'DESC';
		const sort = query.sort ? query.sort : 'rating';
		const whereSearch = {
			ratingbase: 0,
			unrated: 0,
			ratedgames: MoreThan(0),
		}
		if (query.name) whereSearch['name'] = Like(`${query.name}`);
		try {
			const results = await this.ratingRespository.findAndCount({
				select: ['name', 'rating', 'ratedgames', 'maxrating', 'isbot'],
				where: whereSearch,
				order: {
					[sort]: order,
				},
				take: limit,
				skip: limit * page || skip,
			});
			return {
				items: results[0] || [],
				total: results[1] || 0,
				page: page + 1,
				perPage: limit,
				totalPages: Math.ceil(results[1] / limit),
			};
		} catch (error) {
			console.error(error);
			this.logger.error(error);
			throw new HttpException("Error getting all ratings. ", 500, {cause: error});
		}
	}

	public async getPlayersRating(name: string): Promise<any> {
		try {
			const result = await this.ratingRespository.findOne({
				select: ['name', 'rating', 'ratedgames', 'maxrating', 'isbot'],
				where: { name: name }
			});
			if (!result) {
				throw new HttpException("Error: User not found", 404, {cause: new Error("User not found")});
			}
			return result;
		} catch (error) {
			this.logger.error(error);
			throw new HttpException("Error getting players rating. ", 500, {cause: error});
		}
	}

	public async generateRating() {
		let has_error = false;
		let previous = '0 0';
		try {
			await access(process.env.PREVIOUS_FILE);
			previous = await readFile(process.env.PREVIOUS_FILE, 'utf-8');
		} catch (error) {
			this.logger.error(error);
			return;
		}
		const previousData = previous.split(' ');
		let lastUsedGame = previousData[0];
		const DATE_NOW = Date.now();
		const RECENT_LIMIT = DATE_NOW - 1000 * 60 * 60 * 6;

		//Rating calculation parameters:
		const INITIAL_RATING = 1000;
		const BONUS_RATING = 750;
		const BONUS_FACTOR = 60;
		const PARTICIPATION_LIMIT = 10;
		const PARTICIPATION_CUTOFF = 1500;
		const MAX_DROP = 200;
		const RATING_RETENTION = 1000 * 60 * 60 * 24 * 240;

		const playersArray = [];

		const playerRunner = this.playersRepository.manager.connection.createQueryRunner();
		const gameRunner = this.gamesRepository.manager.connection.createQueryRunner();
		await playerRunner.connect();
		await gameRunner.connect();
		await playerRunner.startTransaction();
		await gameRunner.startTransaction();
		try {
			// get al the players
			const getPlayersQuery =
				'select id,name,ratingbase,unrated,isbot,rating,boost,ratedgames,maxrating,ratingage,fatigue FROM players WHERE ratingbase = 0 AND unrated = 0;';
			const p = await this.playersRepository.query(getPlayersQuery);
			for (let i = 0; i < p.length; i++) {
				p[i].fatigue = p[i].fatigue.replace(/\\/g, '');
				// if fatigure starts with " then remove it
				if (p[i].fatigue.startsWith('"'))
					p[i].fatigue = p[i].fatigue.slice(1);
				// if fatigure ends with " then remove it
				if (p[i].fatigue.endsWith('"'))
					p[i].fatigue = p[i].fatigue.slice(0, -1);
				p[i].fatigue = JSON.parse(p[i].fatigue);
				p[i].changed = false;
				playersArray.push(p[i]);
			}

			// get all the games
			const gamesQuery = `
				SELECT id, date, player_white, player_black, result, unrated, size, timertime, timerinc, pieces, capstones, length(notation) as notationlength FROM games 
				where date>1461430800000 and id > ${lastUsedGame} 
				order by id asc limit 50000;`;
			const g = await gameRunner.manager.query(gamesQuery);

			// start transaction
			let updating = true;

			// update game query
			async function updateGame(rw, rb, rwa, rba, id) {
				const updateGameQuery = `UPDATE games SET 
				rating_white=?, rating_black=?, rating_change_white=?, rating_change_black=? where id=?;`;
				try {
					await gameRunner.manager.query(updateGameQuery, [rw, rb, rwa, rba, id ]);
				} catch (error) {
					has_error = true;
					throw new Error(error);
				}
			}

			// Update games loop
			this.logger.debug('Updating games');
			for (let i = 0; i < g.length; i++) {
				const game = g[i];
				let player_white =
					playersArray[
						playersArray.findIndex(
							(p) => p.name == game.player_white,
						)
					];
				let player_black =
					playersArray[
						playersArray.findIndex(
							(p) => p.name == game.player_black,
						)
					];
				let rtw = 0;
				let rtb = 0;
				let artw = 0;
				let artb = 0;
				if (player_white) {
					rtw = player_white.rating;
					artw = this.adjustedRating(
						player_white,
						game.date,
						PARTICIPATION_CUTOFF,
						RATING_RETENTION,
						MAX_DROP,
						PARTICIPATION_LIMIT,
					);
				}
				if (player_black) {
					rtb = player_black.rating;
					artb = this.adjustedRating(
						player_black,
						game.date,
						PARTICIPATION_CUTOFF,
						RATING_RETENTION,
						MAX_DROP,
						PARTICIPATION_LIMIT,
					);
				}
				const quickresult = {'R-0': 1, 'F-0': 1, '1-0': 1, '0-R': 0, '0-F': 0, '0-1': 0, '1/2-1/2': 0.5}[game.result];
				if (player_white && player_black && this.isGameEligible(game) && game.unrated == 0 && player_white != player_black) {
					if (updating) {
						if (game.notationlength > 6) {
							lastUsedGame = game.id;
							if (quickresult === undefined) {
								await updateGame(Math.floor(artw), Math.floor(artb), -2000, -2000, game.id);
							} else {
								const sw = Math.pow(10, rtw / 400);
								const sb = Math.pow(10, rtb / 400);
								const expected = sw / (sw + sb);
								const fairness = expected * (1 - expected);
								const fatiguefactor =
									(1 -(player_white.fatigue[player_black.id] || 0) * 0.4) *
									(1 -(player_black.fatigue[player_white.id] || 0) * 0.4);
								player_white = this.adjustPlayer(
									player_white,
									quickresult - expected,
									fairness,
									fatiguefactor,
									game.date,
									BONUS_FACTOR,
									BONUS_RATING,
									RATING_RETENTION,
									INITIAL_RATING,
								);
								player_black = this.adjustPlayer(
									player_black,
									expected - quickresult,
									fairness,
									fatiguefactor,
									game.date,
									BONUS_FACTOR,
									BONUS_RATING,
									RATING_RETENTION,
									INITIAL_RATING,
								);
								player_white = this.updateFatigue(
									player_white,
									player_black.id.toString(),
									fairness * fatiguefactor,
								);
								player_black = this.updateFatigue(
									player_black,
									player_white.id.toString(),
									fairness * fatiguefactor,
								);
								const artw2 = this.adjustedRating(
									player_white,
									game.date,
									PARTICIPATION_CUTOFF,
									RATING_RETENTION,
									MAX_DROP,
									PARTICIPATION_LIMIT,
								);
								const artb2 = this.adjustedRating(
									player_black,
									game.date,
									PARTICIPATION_CUTOFF,
									RATING_RETENTION,
									MAX_DROP,
									PARTICIPATION_LIMIT,
								);
								await updateGame(
									Math.floor(artw),
									Math.floor(artb),
									Math.round((artw2 - artw) * 10),
									Math.round((artb2 - artb) * 10),
									game.id,
								);
							}
						} else {
							if (
								quickresult === undefined &&
								game.date > RECENT_LIMIT
							) {
								updating = false;
							} else {
								lastUsedGame = game.id;
								await updateGame(Math.floor(artw), Math.floor(artb), -2000, -2000, game.id);
							}
						}
					}
				} else {
					if (updating) {
						lastUsedGame = game.id;
					}
					await updateGame(Math.floor(artw), Math.floor(artb), -2000, -2000, game.id);
				}
			}

			async function updatePlayer(r, b, rg, mr, ra, f, id) {
				// update player query
				const updatePlayerQuery = `UPDATE players SET
				rating=?, boost=?, ratedgames=?, maxrating=?, ratingage=?, fatigue=? where id=?;`;
				try {
					await playerRunner.manager.query(updatePlayerQuery, [
						r,
						b,
						rg,
						mr,
						ra,
						JSON.stringify(f),
						id,
					]);
				} catch (error) {
					has_error = true;
					throw new Error(error);
				}
			}
			this.logger.debug('Updating players');
			for (let i = 0; i < playersArray.length; i++) {
				if (playersArray[i].changed) {
					await updatePlayer(
						playersArray[i].rating,
						playersArray[i].boost,
						playersArray[i].ratedgames,
						playersArray[i].maxrating,
						playersArray[i].ratingage,
						JSON.stringify(playersArray[i].fatigue),
						playersArray[i].id,
					);
				}
			}
			await playerRunner.commitTransaction();
			await gameRunner.commitTransaction();
		} catch (error) {
			this.logger.error("Error processing player and game ratings, rolling back transaction. ", error);
			playerRunner.rollbackTransaction();
			gameRunner.rollbackTransaction();
		} finally {
			playerRunner.release();
			gameRunner.release();
			if (!has_error) {
				this.logger.debug('Finished updating ratings');
				await writeFile('previous.txt', lastUsedGame + ' ' + 'hash');
			}
		}
	}

	// returns the adjusted rating of a player
	public adjustPlayer(
		player: any,
		amount: number,
		fairness: number,
		fatiguefactor: number,
		date: number,
		BONUS_FACTOR: number,
		BONUS_RATING: number,
		RATING_RETENTION: number,
		INITIAL_RATING: number,
	) {
		const bonus = Math.min(
			Math.max(0, (fatiguefactor * amount * Math.max(player.boost, 1) * BONUS_FACTOR) / BONUS_RATING),
			player.boost,
		);
		player.boost -= bonus;
		const k =
			10 +
			15 * Math.pow(0.5, player.ratedgames / 200) +
			15 * Math.pow(0.5, (player.maxrating - INITIAL_RATING) / 300);
		player.rating += fatiguefactor * amount * k + bonus;
		if (player.ratingage == 0) {
			player.ratingage = date - RATING_RETENTION;
		}
		let participation = 20 * Math.pow(0.5, (date - player.ratingage) / RATING_RETENTION);
		participation += fairness * fatiguefactor;
		participation = Math.min(20, participation);
		player.ratingage = Math.log2(participation / 20) * RATING_RETENTION + date;
		player.ratedgames++;
		player.maxrating = Math.max(player.maxrating, player.rating);
		player.changed = true;
		return player;
	}

	// returns the new fatigue value
	public updateFatigue(player: any, opid: string, gamefactor: number) {
		const MULTIPLIER = 1 - gamefactor * 0.4;
		// check if player fatigue is a string adn throw error
		if (typeof player.fatigue == 'string') {
			throw new Error('Fatigue is a string, needs to be an object');
		}
		for (const a in player.fatigue) {
			player.fatigue[a] *= MULTIPLIER;
			if (a != opid && player.fatigue[a] < 0.01) {
				delete player.fatigue[a];
			}
		}
		return (player.fatigue[opid] = (player.fatigue[opid] || 0) + gamefactor);
	}

	public adjustedRating(
		player: any,
		date: number,
		PARTICIPATION_CUTOFF: number,
		RATING_RETENTION: number,
		MAX_DROP: number,
		PARTICIPATION_LIMIT: number ,
	): number {
		if (player.rating < PARTICIPATION_CUTOFF) {
			return player.rating;
		}
		const participation =
			20 * Math.pow(0.5, (date - player.ratingage) / RATING_RETENTION);
		if (player.rating < PARTICIPATION_CUTOFF + MAX_DROP) {
			return Math.min(
				player.rating,
				PARTICIPATION_CUTOFF +
					(MAX_DROP * participation) / PARTICIPATION_LIMIT,
			);
		} else {
			return Math.min(
				player.rating,
				player.rating -
					MAX_DROP * (1 - participation / PARTICIPATION_LIMIT),
			);
		}
	}

	// return true of false if game is eligible
	public isGameEligible(game: any): boolean {
		let eligible = game.size >= 5;
		const limits = [
			null,
			null,
			null,
			null,
			null,
			[180, 20, 32, 1, 1],
			[240, 25, 40, 1, 2],
			[300, 30, 48, 1, 2],
			[360, 40, 64, 1, 3],
		][game.size] || [180, 20, 32, 1, 1];
		if (game.pieces != -1) {
			eligible = eligible && game.pieces >= limits[1];
			eligible = eligible && game.pieces <= limits[2];
			eligible = eligible && game.capstones >= limits[3];
			eligible = eligible && game.capstones <= limits[4];
		}
		if (game.timertime > 0) {
			eligible =
				eligible &&
				limits[0] * 3 <= game.timertime * 3 + game.timerinc * limits[0];
			eligible = eligible && game.timertime >= 60;
		}
		return eligible;
	}
}
