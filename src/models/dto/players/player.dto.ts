// Fatigue contains past games played by the player with the 
// opponent id as the key and the calculated game factor as the value
export type Fatigue = {
	[key: string]: number;
}

export class Player {
	id: number;
	name: string;
	rating: number;
	boost: number;
	maxrating: number;
	ratedgames: number;
	isbot: boolean;
	ratingage: number;
	ratingbase: number;
	participation_rating: number;
	fatigue: Fatigue | string | null;
	changed?: boolean;
}