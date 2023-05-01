import { BadRequestException, Injectable, Logger, NotFoundException, NotImplementedException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { Tournament } from './entities/tournament.entity';
import { TournamentsList, TournamentsQuery } from './dto/tournaments.dto';
import { GameUpdateDto } from './dto/gameUpdate.dto';
import { Game } from './entities/game.entity';
@Injectable()
export class TournamentsService {
	private readonly logger = new Logger(TournamentsService.name);

	constructor(
		@InjectRepository(Tournament)
		private tournaments: Repository<Tournament>,
		@InjectRepository(Game)
		private games: Repository<Game>,
	) {}

	generateSearchQuery(query: TournamentsQuery) {
		const search = {};
		const finished: boolean = query['finished'];
		if (finished == true || finished == false) {
			search['finished'] == finished
		}
		
		return search;
	}

	async getAll(query?: TournamentsQuery): Promise<TournamentsList> {
		const search =this.generateSearchQuery(query);
		try {
			const dbQuery = this.tournaments
				.createQueryBuilder()
				.select('*')
				.where(search)
				.orderBy('id');
			const result = await dbQuery.execute();

			return {
				tournaments: result ?? [],
			};
		} catch (error) {
			this.logger.error(error);
			throw new Error('Could not get tournaments. ' + error);
		}
	}

	async getOneByID(id: number): Promise<Tournament> {
		if (!Number.isSafeInteger(id)) {
			throw new Error(`id must be an integer, but was '${id}'`);
		}
		return await this.tournaments
			.findOneByOrFail({ id });
	}

	async getGameBySeekUid(seekUid: string): Promise<Game> {
		try {
			return await this.games.findOneByOrFail( { seekUid });
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no game with seekUid='${seekUid}'`, { cause });
				this.logger.warn(exc)
				throw exc;
			}
			throw cause;
		}
	}

	async getGameByPlaytakId(playtakId: number): Promise<Game> {
		try {
			return await this.games.findOneByOrFail( { playtakId });
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no game with playtakId='${playtakId}'`, { cause });
				this.logger.warn(exc)
				throw exc;
			}
			throw cause;
		}
	}

	async handleGameUpdate(gameUpdate: GameUpdateDto): Promise<void> {
		if (gameUpdate?.game?.tournament != true) {
			this.logger.debug("Ignoring GameUpdate for non-tournament game", gameUpdate);
			throw new NotImplementedException("Non-tournament games are currently ignored")
		}
		if (!gameUpdate.game.seekUid) {
			throw new BadRequestException("GameUpdate.game.seekUid must be set");
		}
		if (!gameUpdate.game.id) {
			throw new BadRequestException("GameUpdate.game.id must be set");
		}

		switch (gameUpdate.type) {
			case "game.created": {
				this.logger.log(`Game #${gameUpdate.game.id} started: ${gameUpdate.game.white} vs ${gameUpdate.game.black}`);
				
				const game = await this.getGameBySeekUid(gameUpdate.game.seekUid);
				// Mark as "in progress"
				this.games.update(game.id, { playtakId: gameUpdate.game.id });

				break;
			}
			case "game.ended": {
				this.logger.log(`Game #${gameUpdate.game.id} ended: ${gameUpdate.game.white} vs ${gameUpdate.game.black}`);
				
				if (!gameUpdate.game.result) {
					throw new BadRequestException("GameUpdate.game.result must be set when reporting a finished game");
				}
				if (gameUpdate.game.moves == null || gameUpdate.game.moves == undefined) {
					throw new BadRequestException("GameUpdate.game.moves must be set when reporting a finished game");
				}

				const game = await this.getGameByPlaytakId(gameUpdate.game.id);

				if (gameUpdate.game.moves.length === 0) {
					// No moves played => reset state of the game
					this.logger.debug(`Resetting game id=${game.id} since the game ended with no moves played`);
					this.games.update(game.id, { result: null, playtakId: null, seekUid: null });
				}
				else {
					// Mark as "finished"
					const { white, black, result } = gameUpdate.game;
					this.logger.debug(`Marking game id=${game.id} ${white} vs ${black} as finished ${result} (playtakId=${game.playtakId})`);
					this.games.update(game.id, { result: result, playtakId: gameUpdate.game.id })
				}
				break;
			}
			default:
				throw new NotImplementedException(`Cannot handle GameUpdate.type '${gameUpdate.type}'`);
		}
	}
}