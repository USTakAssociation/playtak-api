import { Tournament } from "../entities/tournament.entity";
import { TournamentDto } from "./tournaments.dto";

export function mapToId<T>(obj: T, fieldName: keyof T): T & Record<typeof fieldName, number>{
	const id = obj[fieldName]['id'] as number;
	return {
		...obj,
		[fieldName]: id,
	}
}

export function createMapToId<T>(fieldName: keyof T) {
	return (obj: T) => mapToId(obj, fieldName)
}

// export function mapRecursive<T>(obj: [string, typeof mapToId])


export function tournamentToDto(tournament: Tournament): TournamentDto {
	return {
		...tournament,
		stages: tournament.stages.map(stage => ({
			...stage,
			tournament: tournament.id,
			groups: stage.groups.map(group => ({
				...group,
				stage: stage.id,
				matchups: group.matchups.map(matchup => ({
					...matchup,
					group: group.id,
					games: matchup.games.map(game => ({
						...game,
						matchup: matchup.id
					}))
				}))
			}))
		}))
	}
}