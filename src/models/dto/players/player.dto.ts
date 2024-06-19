/* eslint-disable indent */
import { ApiProperty } from "@nestjs/swagger";

// Fatigue type with key of type string and value of type number
export type Fatigue = {
	[key: string]: number;
}

export class Player {
	@ApiProperty()
	id: number;

	@ApiProperty()
	name: string;

	@ApiProperty()
	rating: number;

	@ApiProperty()
	boost: number;

	@ApiProperty()
	maxrating: number;

	@ApiProperty()
	ratedgames: number;

	@ApiProperty()
	isbot: boolean;

	@ApiProperty()
	ratingage: number;

	@ApiProperty()
	ratingbase: number;
		
	@ApiProperty()
	fatiguerating: number;
		
	@ApiProperty()
	fatigue: Fatigue | string | null;

	@ApiProperty()
	changed?: boolean;

}