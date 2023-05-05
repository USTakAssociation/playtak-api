import { ApiProperty } from "@nestjs/swagger";
import { IsBoolean, IsOptional, IsString } from "class-validator";
import { StageDto } from "./stage.dto";

export class TournamentsList {
	@ApiProperty()
		tournaments: Array<TournamentDto>;
}

export class CreateTournamentDto {
	@IsString()
	@ApiProperty()
		name: string

	@IsString()
	@ApiProperty()
		description: string

	@IsBoolean()
	@IsOptional()
	@ApiProperty({ default: false })
		finished = false
}

export class TournamentDto extends CreateTournamentDto{
	@ApiProperty()
		id: number

	@ApiProperty()
		stages: Array<StageDto>
}

export class TournamentsQuery {
	@IsBoolean()
	@IsOptional()
	@ApiProperty({
		description: 'True if the tournament needs to be over, false if it has to be currently open, null/undefined to ignore this filter.',
		required: false
	})
		finished?: boolean;
	
	// additional search options to consider: participant name, tournament name, date range, ...
}

