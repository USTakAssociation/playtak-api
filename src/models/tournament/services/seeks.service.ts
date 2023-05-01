import { HttpService } from '@nestjs/axios';
import { HttpException, HttpStatus, Injectable, Logger } from '@nestjs/common';
import { URL } from 'url';
import { SeekDto } from '../dto/seek.dto';
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
}
