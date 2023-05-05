import { ApiProperty } from "@nestjs/swagger";
import { IsNumber, IsOptional, IsString } from "class-validator";
import { MatchupDto } from "./matchups.dto";
import { StageDto } from "./stage.dto";

class BaseGroupDto {
	@IsString()
	@IsOptional()
	@ApiProperty()
		name?: string
}

export class CreateGroupDto extends BaseGroupDto {
	@IsNumber()
	@ApiProperty()
		stage: number
}

export class GroupDto extends BaseGroupDto {
	@ApiProperty()
		id: number

	@ApiProperty()
		matchups?: Array<MatchupDto>

	@ApiProperty()
		stage: StageDto|number
}
