import { Injectable, Logger, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { EntityNotFoundError, Repository } from 'typeorm';
import { CreateTournamentDto, TournamentDto, TournamentsList, TournamentsQuery } from '../dto/tournaments.dto';
import { Tournament } from '../entities/tournament.entity';
@Injectable()
export class TournamentsService {
	private readonly logger = new Logger(TournamentsService.name);

	constructor(
		@InjectRepository(Tournament)
		private tournaments: Repository<Tournament>,
	) { }

	generateSearchQuery(query: TournamentsQuery) {
		const search = {};
		const { finished } = query;
		if (finished == true || finished == false) {
			search['finished'] = finished
		}

		return search;
	}

	async getAll(query?: TournamentsQuery): Promise<TournamentsList> {
		const search = this.generateSearchQuery(query);
		try {
			const result = await this.tournaments.find({
				where: search,
				order: { 'id': 1 }
			});

			return {
				tournaments: result ?? [],
			};
		} catch (error) {
			this.logger.error(error);
			throw new Error('Could not get tournaments. ' + error);
		}
	}

	async getEntireTournamentById(id: number): Promise<TournamentDto> {
		if (!Number.isSafeInteger(id)) {
			throw new Error(`id must be an integer, but was '${id}'`);
		}

		try {
			return await this.tournaments
				.findOneOrFail({
					where: { id },
					relations: {
						stages: {
							rules: true,
							groups: {
								matchups: {
									games: {
										rules: true
									}
								}
							}
						}
					}
				});
		}
		catch (cause) {
			if (cause instanceof EntityNotFoundError) {
				const exc = new NotFoundException(`Found no tournament with id='${id}'`, { cause });
				this.logger.warn(exc)
				throw exc;
			}
			throw cause;
		}
	}

	async createTournament(tournament: CreateTournamentDto): Promise<TournamentDto> {
		const tournamentsInstance = this.tournaments.create(tournament);
		return await this.tournaments.manager.save(tournamentsInstance);
	}
}
