import { ApiProperty } from "@nestjs/swagger";

export class TournamentsList {
	@ApiProperty()
		tournaments: Array<Tournament>;
}

export class Tournament {
	@ApiProperty()
		id: number
	@ApiProperty()
		name: string
	@ApiProperty()
		description: string
	@ApiProperty()
		stages: Array<Stage>
	@ApiProperty()
		finished: boolean
}

export class Stage {
	@ApiProperty()
		name?: string
}

export class TournamentsQuery {
	@ApiProperty({
		description: 'True if the tournament needs to be over, false if it has to be currently open, null/undefined to ignore this filter.',
		required: false
	})
		finished?: boolean;
	
	// additional search options to consider: participant name, tournament name, date range, ...
}
