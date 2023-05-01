import { ApiProperty } from "@nestjs/swagger";

type GameUpdateType = "game.created" | "game.ended";

class GameDto {
	@ApiProperty()
		id: number;

	/** UUID */
	@ApiProperty()
		seekUid: string;
	@ApiProperty()
		capstones: number;
	@ApiProperty()
		komi: number;
	@ApiProperty()
		moves?: Array<string>;
	@ApiProperty()
		pieces: number;
	@ApiProperty()
		white: string;
	@ApiProperty()
		black: string;
	@ApiProperty()
		result?: string;
	@ApiProperty()
		boardSize: number;
	@ApiProperty()
		timeContingent: number;
	@ApiProperty()
		timeIncrement: number;
	@ApiProperty()
		tournament: boolean;
	@ApiProperty()
		unrated: boolean;
	@ApiProperty()
		extraTimeAmount: number;
	@ApiProperty()
		extraTimeTriggerMove: number;
}

export class GameUpdateDto {
	@ApiProperty()
		type: GameUpdateType;
	@ApiProperty()
		game: GameDto;
}
