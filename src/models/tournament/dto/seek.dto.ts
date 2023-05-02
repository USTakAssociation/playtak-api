import { ApiProperty } from '@nestjs/swagger';

/** Values come from playtak server, do not change. */
export enum Color {
	White = 'WHITE',
	Black = 'BLACK',
	Any = 'ANY',
}

export class CreateSeekDto {
	/** Refers to `game.entity id` */
	@ApiProperty()
		pntId: number;

	@ApiProperty()
		creator: string;

	@ApiProperty()
		opponent: string;

	/** Seconds */
	@ApiProperty()
		timeContingent: number;

	/** Seconds */
	@ApiProperty()
		timeIncrement: number;

	@ApiProperty()
		extraTimeTriggerMove: number;

	/** Seconds */
	@ApiProperty()
		extraTimeAmount: number;

	/** Float */
	@ApiProperty()
		komi: number;

	@ApiProperty()
		boardSize: number;

	@ApiProperty()
		capstones: number;

	@ApiProperty()
		pieces: number;

	@ApiProperty()
		unrated: boolean;

	@ApiProperty()
		tournament: boolean;

	@ApiProperty({
		enum: Color,
		enumName: 'Color',
	})
		color: Color;
}

export class SeekDto extends CreateSeekDto {
	@ApiProperty()
		id?: number;
}
