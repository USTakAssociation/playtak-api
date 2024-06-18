import { ApiProperty } from '@nestjs/swagger';
import { IsInt, IsString } from 'class-validator';

export class GameRulesQuery {
}

export class CreateGameRulesDto {
	@IsString()
	@ApiProperty()
		name: string;

	/** Seconds */
	@IsInt()
	@ApiProperty()
		timeContingent: number;

	/** Seconds */
	@IsInt()
	@ApiProperty()
		timeIncrement: number;

	@IsInt()
	@ApiProperty()
		extraTimeTriggerMove: number;

	/** Seconds */
	@IsInt()
	@ApiProperty()
		extraTimeAmount: number;

	@IsInt()
	@ApiProperty()
		komi: number;

	@IsInt()
	@ApiProperty()
		boardSize: number;

	@IsInt()
	@ApiProperty()
		capstones: number;

	@IsInt()
	@ApiProperty()
		pieces: number;
}

export class GameRulesDto extends CreateGameRulesDto {
	@IsInt()
	@ApiProperty()
		id: number;
}