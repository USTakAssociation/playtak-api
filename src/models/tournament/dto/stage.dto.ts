import { ApiProperty } from "@nestjs/swagger";
import { IsInt, IsNumber, IsString } from "class-validator";
import { GameRulesDto } from "./gameRules.dto";
import { GroupDto } from "./group.dto";
import { TournamentDto } from "./tournaments.dto";


class BaseStageDto {
	@IsString()
	@ApiProperty()
		name: string
}
export class CreateStageDto extends BaseStageDto{

	@IsNumber()
	@ApiProperty()
		tournament: number
		
	@IsInt()
	@ApiProperty()
		rules: number
}

export class StageDto extends BaseStageDto{
	@ApiProperty()
		id: number

	@ApiProperty()
		groups?: Array<GroupDto>
		
	@ApiProperty()
		tournament: TournamentDto|number
	
	@ApiProperty()
		rules: GameRulesDto|number
}
