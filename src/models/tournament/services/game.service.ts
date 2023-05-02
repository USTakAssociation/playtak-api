import { BadRequestException, Injectable, Logger, NotFoundException, NotImplementedException, PreconditionFailedException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { CreateGameDto, GameDto, GameQuery } from '../dto/game.dto';
import { GameUpdateDto } from '../dto/gameUpdate.dto';
import { Game } from '../entities/game.entity';
import { GameRulesService } from './gameRules.service';
import { MatchupsService } from './matchups.service';
@Injectable()
export class GameService {
	private readonly logger = new Logger(GameService.name);

	constructor(
		@InjectRepository(Game)
		private readonly games: Repository<Game>,
		private readonly matchupService: MatchupsService,
		private readonly gameRulesService: GameRulesService,
	) {}

	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	async getAll(query: GameQuery): Promise<Array<GameDto>> {
		return await this.games.find();
	}

	async getGameById(id: number, relations = { matchup: false, rules: false }): Promise<Game> {
		try {
			this.logger.log("get by id", id)
			return await this.games.findOne({ relations, where: { id } });
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no game with id='${id}'`, { cause });
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

	async moveGameToInProgress(gameId: number, playtakId: number) {
		const game = await this.getGameById(gameId);
		if (game.playtakId) {
			throw new PreconditionFailedException(`Game id=${game.id} cannot be moved to 'in Progress' because playtakId is already set (${playtakId})`);
		}
		this.logger.debug(`Moving game id=${game.id} to 'in Progress' with playtakId=${playtakId}`);
		this.games.update(game.id, { playtakId });
	}

	async resetGameState(gameId: number) {
		const game = await this.getGameById(gameId);
		await this.games.update(game.id, { result: null, playtakId: null });
	}

	async markGameAsFinished(gameId: number, result: string) {
		const game = await this.getGameById(gameId);
		if (game.result) {
			throw new PreconditionFailedException(`Game id=${game.id} cannot be marked as 'finished' with result='${result}' because it is already finished (previous result='${game.result}')`);
		}
		this.logger.log(`Moving game id=${game.id} to 'finished' with result=${result}`);
		this.games.update(game.id, { result });
	}

	async handleGameUpdate(gameUpdate: GameUpdateDto): Promise<void> {
		if (gameUpdate?.game?.tournament != true) {
			this.logger.debug("Ignoring GameUpdate for non-tournament game", gameUpdate);
			throw new NotImplementedException("Non-tournament games are currently ignored")
		}
		if (!gameUpdate.game.pntId) {
			throw new BadRequestException("GameUpdate.game.pntId must be set");
		}
		if (!gameUpdate.game.id) {
			throw new BadRequestException("GameUpdate.game.id must be set");
		}

		switch (gameUpdate.type) {
			case "game.created": {
				this.logger.log(`Game id=${gameUpdate.game.pntId} playtakId=${gameUpdate.game.id} started: ${gameUpdate.game.white} vs ${gameUpdate.game.black}`);
				await this.moveGameToInProgress(gameUpdate.game.pntId, gameUpdate.game.id);
				break;
			}
			case "game.ended": {
				this.logger.log(`Game id=${gameUpdate.game.pntId} playtakId=${gameUpdate.game.id} ended: ${gameUpdate.game.white} vs ${gameUpdate.game.black}`);
				
				if (!gameUpdate.game.result) {
					throw new BadRequestException("GameUpdate.game.result must be set when reporting a finished game");
				}
				if (gameUpdate.game.moves == null || gameUpdate.game.moves == undefined) {
					throw new BadRequestException("GameUpdate.game.moves must be set when reporting a finished game");
				}

				if (gameUpdate.game.moves.length === 0) {
					this.logger.debug(`Resetting game id=${gameUpdate.game.pntId} since it ended with no moves played (playtakId=${gameUpdate.game.id})`);
					await this.resetGameState(gameUpdate.game.pntId);
				}
				else {
					const { white, black, result, pntId: gameId, id: playtakId } = gameUpdate.game;
					this.logger.debug(`Marking game id=${gameId} playtakId=${playtakId} ${white} vs ${black} as finished ${result}`);
					await this.markGameAsFinished(gameId, result);
				}
				break;
			}
			default:
				throw new NotImplementedException(`Cannot handle GameUpdate.type '${gameUpdate.type}'`);
		}
	}

	
	async createGame(gameToCreate: CreateGameDto): Promise<GameDto> {
		const gameRules = await this.gameRulesService.getById(gameToCreate.rules);
		const matchup = await this.matchupService.getById(gameToCreate.matchup);

		const gameInstance = this.games.create({ ...gameToCreate, matchup: matchup, rules: gameRules });
		return await this.games.manager.save(gameInstance);
	}
}
