export default class GameDTO {
	id: number;
	date: Date;
	size: number;
	player_white: string;
	player_black: string;
	notation: string;
	result: string;
	timertime: number;
	timerinc: number;
	rating_white: number;
	rating_black: number;
	unrated: number;
	tournament: number;
	komi: number;
	pieces: number;
	capstones: number;
	rating_change_white: number;
	rating_change_black: number;
}

