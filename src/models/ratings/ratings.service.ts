import { HttpException, Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Like, MoreThan, Repository } from 'typeorm';
import { Ratings } from './entities/ratings.entity';
import { Rating, RatingList, RatingQuery } from '../dto/rating/ratings.dto';
import { access, readFile, writeFile } from 'fs/promises';
import { Players } from './entities/players.entity';
import { Games } from '../games-history/entities/games.entity';
import { Fatigue, Player } from '../dto/players/player.dto';
import { DefaultExceptionDto } from '../dto/error.dto';
import { gzipSync } from 'zlib';
import { writeFileSync } from 'fs';

@Injectable()
export class RatingService {
	private readonly logger = new Logger(RatingService.name);

	constructor(
		@InjectRepository(Ratings, 'default')
		private ratingRepository: Repository<Ratings>,
		@InjectRepository(Players, 'default')
		private playersRepository: Repository<Players>,
		@InjectRepository(Games, 'games')
		private gamesRepository: Repository<Games>,
	) {}

	async getAll(query?: RatingQuery): Promise<RatingList | DefaultExceptionDto> {
		const limit = parseInt(query.limit) || 50;
		const skip = parseInt(query.skip) || 0;
		const page = parseInt(query.page) || 0;
		const order: 'ASC' | 'DESC' = query.order || 'DESC';
		const sort = query.sort ? query.sort : 'participation_rating';
		const whereSearch = {
			ratingbase: 0,
			unrated: 0,
			ratedgames: MoreThan(0),
		}
		if(query.name){ whereSearch['name'] = Like(`${query.name}`); }
		if(query.id){ whereSearch['id'] = query.id; }
		try {
			const results = await this.ratingRepository.findAndCount({
				select: ['name', 'rating', 'ratedgames', 'maxrating', 'participation_rating', 'isbot'],
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
			this.logger.error(error);
			throw new HttpException("Error getting all ratings. ", 500, {cause: error});
		}
	}

	public async getPlayersRating(name: string): Promise<Rating | DefaultExceptionDto> {
		try {
			const result = await this.ratingRepository.findOne({
				select: ['name', 'rating', 'ratedgames', 'participation_rating', 'maxrating', 'isbot'],
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
	
	public parseFatigue(player: Player): Player {
		if (player.fatigue === null) {
			return player;
		  }
		
		if (typeof player.fatigue === 'string') {
			let fatigueStr: string = player.fatigue;
			
			// Clean up the string
			fatigueStr = fatigueStr.replace(/\\/g, '');
			fatigueStr = fatigueStr.startsWith('"') ? fatigueStr.slice(1) : fatigueStr;
			fatigueStr = fatigueStr.endsWith('"') ? fatigueStr.slice(0, -1) : fatigueStr;
			
			try {
				player.fatigue = JSON.parse(fatigueStr) as Fatigue;
			} catch (error) {
				console.error(`Failed to parse fatigue for player ${player.id}:`, error);
				player.fatigue = null;
			}
		}
	
		return player;
	}

	// generates the games and players ratings
	// the rating calculation is based on Nohatcoders Elo rating system
	// https://nohatcoder.dk/2015-03-22-1.html
	// this function gets the games based on the last used game id from the previous.txt file
	// it gets all the players from the players table
	// this generateRating function does 3 database transactions
	// it loops through the games data and updates the players rating for that game and sets if the player has changed
	// it loops through the players data and updates the players rating for that player if they have changed
	// it updates the players fatigue rating even if they haven't changed
	// it tries to commit the transactions and if there is an error it rolls back the transactions
	// it creates a rating.json file for the ratings.html page
	// if there is no error it writes the last used game id to the previous.txt file
	public async generateRating(): Promise<void> {
		let has_error = false;
		let previous = '0 0';
		try {
			await access(process.env.PREVIOUS_FILE);
			previous = await readFile(process.env.PREVIOUS_FILE, 'utf-8');
		} catch (error) {
			this.logger.error(error);
			throw new Error(error);
		}
		const previousData = previous.split(' ');
		let lastUsedGame = previousData[0];
		const DATE_NOW = Date.now();
		const RECENT_LIMIT = DATE_NOW - 1000 * 60 * 60 * 6;

		// Rating calculation parameters:
		const INITIAL_RATING = 1000;
		const BONUS_RATING = 750;
		const BONUS_FACTOR = 60;
		const PARTICIPATION_LIMIT = 10;
		const PARTICIPATION_CUTOFF = 1500;
		const MAX_DROP = 200;
		const RATING_RETENTION = 1000 * 60 * 60 * 24 * 240;

		let playersArray: Array<Player> = [];
		const playersRatingList=[]
		const playerRunner = this.playersRepository.manager.connection.createQueryRunner();
		const gameRunner = this.gamesRepository.manager.connection.createQueryRunner();
		await playerRunner.connect();
		await gameRunner.connect();
		await playerRunner.startTransaction();
		await gameRunner.startTransaction();
		try {
			// get all the players
			const getPlayersQuery =
				'select id,name,ratingbase,unrated,isbot,rating,boost,ratedgames,maxrating,ratingage,fatigue FROM players WHERE ratingbase = 0 AND unrated = 0;';
			const playersData: Array<Player> = await this.playersRepository.query(getPlayersQuery);
			// loop through the players to parse the players fatigue, add a changed boolean, and push them to the playersArray
			playersArray = playersData.map(player => ({
				...player,
				changed: false,
			  })).map(this.parseFatigue);

			// get all the games
			const gamesQuery = `
				SELECT id, date, player_white, player_black, result, unrated, size, timertime, timerinc, pieces, capstones, length(notation) as notationlength FROM games 
				where date>1461430800000 and id > ${lastUsedGame} 
				order by id asc limit 50000;`;
			const gamesData = await gameRunner.manager.query(gamesQuery);

			// set game updating flag
			let updating = true;

			// update game query function
			async function updateGame(whiteRating: number, blackRating: number, whiteRatingAdjusted: number, blackRatingAdjusted: number, id: number): Promise<void> {
				const updateGameQuery = `UPDATE games SET 
				rating_white=?, rating_black=?, rating_change_white=?, rating_change_black=? where id=?;`;
				try {
					await gameRunner.manager.query(updateGameQuery, [whiteRating, blackRating, whiteRatingAdjusted, blackRatingAdjusted, id ]);
				} catch (error) {
					has_error = true;
					this.logger.error('Error updating game. ', error);
					throw new Error(error);
				}
			}

			// Update games loop
			this.logger.debug('Updating games');
			for (let i = 0; i < gamesData.length; i++) {
				const game = gamesData[i];
				// get the player white and player black from the playersArray
				let player_white: Player =
					playersArray[
						playersArray.findIndex((p) => p.name == game.player_white)
					];
				let player_black: Player =
					playersArray[
						playersArray.findIndex((p) => p.name == game.player_black)
					];
				let whiteRating = 0;
				let blackRating = 0;
				let whiteAdjustedRating = 0;
				let blackAdjustedRating = 0;
				// check if player_white exists and set whiteRating and whiteAdjustedRating
				if (player_white) {
					whiteRating = player_white.rating;
					whiteAdjustedRating = this.adjustedRating(
						player_white,
						game.date,
						PARTICIPATION_CUTOFF,
						RATING_RETENTION,
						MAX_DROP,
						PARTICIPATION_LIMIT,
					);
				}
				// check if player_black exists and set blackRating and blackAdjustedRating
				if (player_black) {
					blackRating = player_black.rating;
					blackAdjustedRating = this.adjustedRating(
						player_black,
						game.date,
						PARTICIPATION_CUTOFF,
						RATING_RETENTION,
						MAX_DROP,
						PARTICIPATION_LIMIT,
					);
				}
				const quickResult = {'R-0': 1, 'F-0': 1, '1-0': 1, '0-R': 0, '0-F': 0, '0-1': 0, '1/2-1/2': 0.5}[game.result];
				if (player_white && player_black && this.isGameEligible(game) && game.unrated == 0 && player_white != player_black) {
					if (updating) {
						// checks if there are more than 6 moves in the game
						if (game.notationlength > 6) {
							lastUsedGame = game.id;
							if (quickResult === undefined) {
								await updateGame(Math.floor(whiteAdjustedRating), Math.floor(blackAdjustedRating), -2000, -2000, game.id);
							} else {
								const sw = Math.pow(10, whiteRating / 400);
								const sb = Math.pow(10, blackRating / 400);
								const expected = sw / (sw + sb);
								const fairness = expected * (1 - expected);
								const fatigueFactor =
									(1 -(player_white.fatigue[player_black.id] || 0) * 0.4) *
									(1 -(player_black.fatigue[player_white.id] || 0) * 0.4);
								player_white = this.adjustPlayer(
									player_white,
									quickResult - expected,
									fairness,
									fatigueFactor,
									game.date,
									BONUS_FACTOR,
									BONUS_RATING,
									RATING_RETENTION,
									INITIAL_RATING,
								);
								player_black = this.adjustPlayer(
									player_black,
									expected - quickResult,
									fairness,
									fatigueFactor,
									game.date,
									BONUS_FACTOR,
									BONUS_RATING,
									RATING_RETENTION,
									INITIAL_RATING,
								);
								player_white = this.updateFatigue(
									player_white,
									`${player_black.id}`,
									fairness * fatigueFactor,
								);
								player_black = this.updateFatigue(
									player_black,
									`${player_white.id}`,
									fairness * fatigueFactor,
								);
								const whiteAdjustedRating2 = this.adjustedRating(
									player_white,
									game.date,
									PARTICIPATION_CUTOFF,
									RATING_RETENTION,
									MAX_DROP,
									PARTICIPATION_LIMIT,
								);
								const blackAdjustedRating2 = this.adjustedRating(
									player_black,
									game.date,
									PARTICIPATION_CUTOFF,
									RATING_RETENTION,
									MAX_DROP,
									PARTICIPATION_LIMIT,
								);
								await updateGame(
									Math.floor(whiteAdjustedRating),
									Math.floor(blackAdjustedRating),
									Math.round((whiteAdjustedRating2 - whiteAdjustedRating) * 10),
									Math.round((blackAdjustedRating2 - blackAdjustedRating) * 10),
									game.id,
								);
							}
						} else {
							if ( quickResult === undefined && game.date > RECENT_LIMIT) {
								updating = false;
							} else {
								lastUsedGame = game.id;
								await updateGame(Math.floor(whiteAdjustedRating), Math.floor(blackAdjustedRating), -2000, -2000, game.id);
							}
						}
					}
				} else {
					if (updating) {
						lastUsedGame = game.id;
					}
					await updateGame(Math.floor(whiteAdjustedRating), Math.floor(blackAdjustedRating), -2000, -2000, game.id);
				}
			}

			// update player query function
			async function updatePlayer(rating: number, boost: number, ratedGames, maxRating, ratingAge, fatigueObject, id, participation_rating) {
				const updatePlayerQuery = `UPDATE players SET
				rating=?, boost=?, ratedgames=?, maxrating=?, ratingage=?, fatigue=?, participation_rating=? where id=?;`;
				try {
					await playerRunner.manager.query(updatePlayerQuery, [
						rating,
						boost,
						ratedGames,
						maxRating,
						ratingAge,
						JSON.stringify(fatigueObject),
						participation_rating,
						id,
					]);
				} catch (error) {
					has_error = true;
					this.logger.error('Error updating player. ', error);
					throw new Error(error);
				}
			}

			// Update players loop
			this.logger.debug('Updating players');
			for (let i = 0; i < playersArray.length; i++) {
				playersArray[i].participation_rating = this.adjustedRating(
					playersArray[i],
					Date.now(),
					PARTICIPATION_CUTOFF,
					RATING_RETENTION,
					MAX_DROP,
					PARTICIPATION_LIMIT,
				);
				if (playersArray[i].changed) {
					await updatePlayer(
						playersArray[i].rating,
						playersArray[i].boost,
						playersArray[i].ratedgames,
						playersArray[i].maxrating,
						playersArray[i].ratingage,
						JSON.stringify(playersArray[i].fatigue),
						playersArray[i].id,
						playersArray[i].participation_rating,
					);
				} else {
					// always update a players participation_rating even if they haven't changed
					const updateParticipationRatingQuery = `UPDATE players SET participation_rating=? where id=?;`;
					await playerRunner.manager.query(updateParticipationRatingQuery, [playersArray[i].participation_rating, playersArray[i].id]);
				}
				if(playersArray[i].ratedgames > 0 || (playersArray[i].unrated && playersArray[i].isbot)){
					playersRatingList.push([
						playersArray[i].name,
						playersArray[i].participation_rating,
						Math.floor(playersArray[i].rating),
						playersArray[i].ratedgames,
						playersArray[i].isbot ? 1 : 0
					])
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

				// sort the playersRatingList by participation_rating
				playersRatingList.sort(function(b,a){return a[1]-b[1]})
				for(let a = 0; a < playersRatingList.length;a++){
					playersRatingList[a][1]=Math.floor(playersRatingList[a][1])
				}
				// write the playersRatingList to a json file
				const jsonList = JSON.stringify(playersRatingList)
				const gzipList = gzipSync(jsonList,{level:9})
				writeFileSync(process.env.RATING_OUTPUT_PATH+'ratinglist.json.gz', gzipList)
			}
		}
	}

	// mutates the player boost, ratingage, maxrating, ratedgames, and changed
	// called when looping through the gamesData
	public adjustPlayer(
		player: Player,
		amount: number,
		fairness: number,
		fatigueFactor: number,
		date: number,
		BONUS_FACTOR: number,
		BONUS_RATING: number,
		RATING_RETENTION: number,
		INITIAL_RATING: number,
	): Player{
		const bonus = Math.min(
			Math.max(0, (fatigueFactor * amount * Math.max(player.boost, 1) * BONUS_FACTOR) / BONUS_RATING),
			player.boost,
		);
		player.boost -= bonus;
		const k =
			10 +
			15 * Math.pow(0.5, player.ratedgames / 200) +
			15 * Math.pow(0.5, (player.maxrating - INITIAL_RATING) / 300);
		player.rating += fatigueFactor * amount * k + bonus;
		if (player.ratingage == 0) {
			player.ratingage = date - RATING_RETENTION;
		}
		let participation = 20 * Math.pow(0.5, (date - player.ratingage) / RATING_RETENTION);
		participation += fairness * fatigueFactor;
		participation = Math.min(20, participation);
		player.ratingage = Math.log2(participation / 20) * RATING_RETENTION + date;
		player.ratedgames++;
		player.maxrating = Math.max(player.maxrating, player.rating);
		player.changed = true;
		return player;
	}

	// mutates the player fatigue object
	// called when looping through the gamesData
	public updateFatigue(player: Player, opponentID: string, gameFactor: number): Player {
		const MULTIPLIER = 1 - gameFactor * 0.4;
		// check if player fatigue is a string and throw error
		if (typeof player.fatigue == 'string') {
			throw new Error('Fatigue is a string, needs to be an object');
		}
		for (const a in player.fatigue) {
			player.fatigue[a] *= MULTIPLIER;
			if (a != opponentID && player.fatigue[a] < 0.01) {
				delete player.fatigue[a];
			}
		}
		player.fatigue[opponentID] = (player.fatigue[opponentID] || 0) + gameFactor;
		return player;
	}

	// returns the players adjusted rating
	// called when looping through the playersData
	public adjustedRating(
		player: Player,
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

	// return true or false if game is eligible for rating
	// called when looping through the gamesData
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
