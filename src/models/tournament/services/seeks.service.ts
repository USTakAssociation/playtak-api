import { HttpService } from '@nestjs/axios';
import { HttpException, HttpStatus, Injectable, Logger } from '@nestjs/common';
import { URL } from 'url';
import { Color, CreateSeekDto, SeekDto } from '../dto/seek.dto';
import { GameService } from './game.service';
import { ConfigService } from '@nestjs/config';
import { InjectRepository } from '@nestjs/typeorm';
import { Game } from '../entities/game.entity';
import { Repository } from 'typeorm';

@Injectable()
export class SeeksService {
	private readonly logger = new Logger(SeeksService.name);

	constructor(
		@InjectRepository(Game)
		private readonly games: Repository<Game>,
		private readonly gameService: GameService,
		private readonly httpService: HttpService,
		private readonly configService: ConfigService,
	) {}

	private getSeekApiUrl() {
		const baseApiUrl = this.configService.getOrThrow<string>('PLAYTAK_API_URL');
		return new URL('/api/v1/seeks', baseApiUrl);
	}

	async getSeeks(): Promise<Array<SeekDto>> {

		const response = await this.httpService.axiosRef.get<Array<SeekDto>>(
			this.getSeekApiUrl().href,
			{
				headers: { 'Accept': 'application/json' },
			}).catch(reason => { throw new HttpException(`Playtak-Server responded with '${reason}'`, HttpStatus.FAILED_DEPENDENCY)});

		if (response.status == HttpStatus.OK) {
			return response.data;
		}

		throw new HttpException(
			'Received an error from playtak-server while fetching seeks', 
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
		// Todo choose creator/opponent from player1/player2 according to who invoked this endpoint

		const game = await this.gameService.getGameById(gameId, { matchup: true, rules: true });

		if (!game.rules) 
			throw TypeError(`Game.rules is not defined for game id=${gameId}`);
		if (!game.matchup) 
			throw TypeError(`Game.matchup is not defined for game id=${gameId}`)

		// eslint-disable-next-line @typescript-eslint/no-unused-vars
		const { name: _, ...rules } =  game.rules;

		const seekInquiry: CreateSeekDto = {
			creator: game.matchup.player1,
			opponent: game.matchup.player2,
			unrated: false,
			tournament: true,
			color: game.player1goesFirst ? Color.White : Color.Black,
			...rules,
		}
		
		const url = this.getSeekApiUrl().href;
		this.logger.debug(`Trying to create seek for game id=${gameId} ${url}`, seekInquiry);

		const response = await this.httpService.axiosRef.put<SeekDto>(
			url,
			seekInquiry,
			{
				headers: {
					'Accept': 'application/json',
					'Content-Type': 'application/json',
				},
			}
		).catch(reason => { throw new HttpException(`Playtak-Server responded with '${reason} ${reason.status}'`, HttpStatus.FAILED_DEPENDENCY)});


		if (response.status == HttpStatus.OK) {
			const seek: SeekDto = response.data;
			this.logger.debug('Created seek', seek);
			if (seek.uid) {
				this.gameService.setSeekUid(gameId, seek.uid);
				return seek;
			}
			throw new HttpException('Created seek did not contain a uid', HttpStatus.FAILED_DEPENDENCY);
		}

		// Todo handle '<creator-name> currently not logged in to playtak' response
		// and display a corresponding message in the UI
		throw new HttpException(
			'Received an error from playtak-server while trying to create seek', 
			HttpStatus.FAILED_DEPENDENCY,
			{
				cause: new HttpException(response.data, response.status)
			}
		);
	}
}
