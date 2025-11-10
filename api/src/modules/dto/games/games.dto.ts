import { ApiProperty } from '@nestjs/swagger';
export class Game {
	id: number;
	capstones: number;
	date: number;
	komi: number;
	notation: string;
	pieces: number;
	player_black: string;
	player_white: string;
	rating_black: number;
	rating_change_black: number;
	rating_change_white: number;
	rating_white: number;
	result: string;
	size: number;
	timerinc: number;
	timertime: number;
	tournament: number;
	unrated: number;
	extra_time_amount: number;
	extra_time_trigger: number;
}

export class GamesList {
	items: Array<Game>;
	total: number;
	perPage: number;
	page: number;
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
		description: `Optional skip if you don't use page`,
		required: false
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
		enum: [3, 4, 5, 6, 7, 8],
		required: false
	})
	size?: 3 | 4 | 5 | 6 | 7 | 8;

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

	@ApiProperty({
		description: 'Komi value (komi is represented as half values 1 = 0.5 komi, 2 = 1.0 komi etc)',
		required: false,
		enum: ['1', '2', '3', '4', '5', '6', '7', '8']
	})
	komi?: string;

	@ApiProperty({
		description: 'Game Time per player in seconds',
		required: false,
		enum: ['60', '120', '180', '300', '420', '600', '900', '1200', '1800', '2700', '3600', '5400', '7200', '10800']
	})
	timertime?: string;

	@ApiProperty({
		description: 'Increment per turn in seconds',
		required: false,
		enum: ['0', '1', '2', '3', '5', '7', '10', '15', '20', '30', '45', '60', '90', '120', '180']
	})
	timerinc?: string;

	@ApiProperty({
		description: 'Extra time amount in seconds',
		required: false,
		enum: ['5', '25', '30', '35', '40', '45', '50', '55', '60', '20']
	})
	extra_time_amount?: string;

	@ApiProperty({
		description: 'What move number to add extra time',
		required: false,
		enum: ['180', '300', '420', '600', '900', '1200', '1800', '2700', '3600']
	})
	extra_time_trigger?: string;
}

export class GamePTN {}

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
