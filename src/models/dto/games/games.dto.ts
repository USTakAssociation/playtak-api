/* eslint-disable indent */
import { ApiProperty } from "@nestjs/swagger";
export class Game {
	@ApiProperty()
	id: number;

	@ApiProperty()
	capstones: number;

	@ApiProperty()
	date:number;

	@ApiProperty()
	komi:number;

	@ApiProperty()
	notation: string;

	@ApiProperty()
	pieces: number;

	@ApiProperty()
	player_black: string;

	@ApiProperty()
	player_white: string;

	@ApiProperty()
	rating_black: number;

	@ApiProperty()
	rating_change_black: number;

	@ApiProperty()
	rating_change_white: number;

	@ApiProperty()
	rating_white: number;

	@ApiProperty()
	result: string;

	@ApiProperty()
	size:number;

	@ApiProperty()
	timerinc: number;

	@ApiProperty()
	timertime: number;

	@ApiProperty()
	tournament: number;

	@ApiProperty()
	unrated: number;

	@ApiProperty()
	extra_time_amount: number;

	@ApiProperty()
	extra_time_trigger: number;
}

export class GamesList {
	@ApiProperty()
	items: Array<Game>;
	
	@ApiProperty()
	total: number;
	
	@ApiProperty()
	perPage: number;
	
	@ApiProperty()
	page: number;
	
	@ApiProperty()
	totalPages: number;
}

export class GameQuery {
	@ApiProperty({
		description: 'Number of results per page',
		required: false
	})
	limit?: string;

	@ApiProperty({
		description: 'page number',
		required: false
	})
	page?: string;

	@ApiProperty({
		description: 'Optional skip if you dont use page',
		required: false,
	})
	skip?: string;

	@ApiProperty({
		description: 'Order By',
		enum: ['ASC', 'DESC'],
		required: false
	})
	order?: 'ASC' | 'DESC';

	@ApiProperty({
		description: 'Column sort',
		required: false
	})
	sort?: string;

	@ApiProperty({
		description: 'Game id',
		required: false
	})
	id?: string;

	@ApiProperty({
		description: 'Player white username',
		required: false
	})
	player_white?: string;
	
	@ApiProperty({
		description: 'Player black username',
		required: false
	})
	player_black?: string;
	
	@ApiProperty({
		description: 'Game result',
		enum: ['X-0', 'R-0', 'F-0', '1-0', '0-X', '0-R', '0-F', '0-1', '1/2-1/2'],
		required: false
	})
	game_result?: string;

	@ApiProperty({
		description: 'Board size',
		enum: [3,4,5,6,7,8],
		required: false
	})
	size?: 3 | 4 | 5| 6| 7 | 8;

	@ApiProperty({
		description: 'Game Type',
		enum: ['normal', 'tournament', 'unrated'],
		required: false
	})
	type?: 'normal' | 'tournament' | 'unrated';

	@ApiProperty({
		description: 'Mirror search results',
		enum: ['true', 'false'],
		required: false
	})
	mirror: string;

}

export class GamePTN {
	
}

export class AnonDetails {
	@ApiProperty()
	atime: string;

	@ApiProperty()
	atimeMs: number;

	@ApiProperty()
	birthtime: string;

	@ApiProperty()
	birthtimeMs: number;

	@ApiProperty()
	blksize: number;

	@ApiProperty()
	blocks: number;

	@ApiProperty()
	ctime: string;

	@ApiProperty()
	ctimeMs: number;

	@ApiProperty()
	dev: number;

	@ApiProperty()
	gid: number;

	@ApiProperty()
	ino: number;

	@ApiProperty()
	mode: number;

	@ApiProperty()
	mtime: string;

	@ApiProperty()
	mtimeMs: number;

	@ApiProperty()
	nlink: number;

	@ApiProperty()
	rdev: number;

	@ApiProperty()
	size: number;

	@ApiProperty()
	uid: number;

}
