import { HttpService } from '@nestjs/axios';
import { HttpException, HttpStatus, Injectable, Logger } from '@nestjs/common';
import { URL } from 'url';
import { Color, CreateSeekDto, SeekDto } from '../dto/seek.dto';
import { GameService } from './game.service';

@Injectable()
export class SeeksService {
	private readonly logger = new Logger(SeeksService.name);

	static readonly seekApiUrl = new URL("/api/v1/seeks", "http://127.0.0.1:9998");

	constructor(
		private readonly gameService: GameService,
		private readonly httpService: HttpService,
	) {}

	async getSeeks(): Promise<Array<SeekDto>> {
		const response = await this.httpService.axiosRef.get<Array<SeekDto>>(
			SeeksService.seekApiUrl.href,
			{
				headers: { 'Accept': 'application/json' },
			});
		if (response.status == HttpStatus.OK) {
			return response.data;
		}

		throw new HttpException(
			"Received an error from playtak-server while fetching seeks", 
			HttpStatus.FAILED_DEPENDENCY,
			{
				cause: new HttpException(response.data, response.status)
			}
		);
	}

	/**
	 * Creates a tournament seek for `gameId` on the playtak server.
	 * Uses the rules and player names associated with the game.
	 * 
	 * Updates the game's seekUid so that `GameUpdates` can be associated with the entity. 
	 */
	async createSeek(gameId: number): Promise<SeekDto> {
		// Todo make sure this can only be called from someone logged in
		const game = await this.gameService.getGameById(gameId);

		if (!game.rules) 
			throw TypeError(`Game.rules is not defined for game id=${gameId}`);
		if (!game.matchup) 
			throw TypeError(`Game.matchup is not defined for game id=${gameId}`)

		const seekInquiry: CreateSeekDto = {
			creator: game.matchup.player1,
			opponent: game.matchup.player2,
			unrated: false,
			tournament: true,
			color: game.player1goesFirst ? Color.White : Color.Black,
			...game.rules,
		}
		this.logger.debug(`Trying to create seek for game id=${gameId}`, seekInquiry);

		const response = await this.httpService.axiosRef.put<SeekDto>(
			SeeksService.seekApiUrl.href,
			seekInquiry,
			{
				headers: {
					'Accept': 'application/json',
					'Content-Type': 'application/json',
				},
			}
		);

		if (response.status == HttpStatus.OK) {
			const seek: SeekDto = response.data;
			this.logger.debug("Created seek", seek);
			if (seek.uid) {
				this.gameService.setSeekUid(gameId, seek.uid);
				return seek;
			}
			throw new HttpException("Created seek did not contain a uid", HttpStatus.FAILED_DEPENDENCY);
		}

		// Todo handle "<creator-name> currently not logged in to playtak" response
		// and display a corresponding message in the UI
		throw new HttpException(
			"Received an error from playtak-server while trying to create seek", 
			HttpStatus.FAILED_DEPENDENCY,
			{
				cause: new HttpException(response.data, response.status)
			}
		);
	}
}
