import { ApiProperty } from "@nestjs/swagger";

export class Rating {
	id: number;
	name: string;
	rating: number;
	maxrating: number;
	ratedgames: number;	
	participation_rating: number;
	isbot: boolean;
}

export class RatingList {
	items: Array<Rating>;
	total: number;
	perPage: number;
	page: number;
	totalPages: number;
}

export class RatingQuery {
	@ApiProperty({
		description: 'Number of results per page for pagination. Optional.',
		required: false
	})
	limit?: string;

	@ApiProperty({
		description: 'Page number for pagination. Optional.',
		required: false
	})
	page?: string;

	@ApiProperty({
		description: 'Number of items to skip for pagination. Can use this instead of a page number. Optional.',
		required: false,
	})
	skip?: string;

	@ApiProperty({
		description: 'Order By either ASC or DESC for sorting. Optional.',
		enum: ['ASC', 'DESC'],
		required: false
	})
	order?: 'ASC' | 'DESC';

	@ApiProperty({
		description: 'Which column to sort by for sorting. Optional.',
		required: false
	})
	sort?: string;

	@ApiProperty({
		description: 'Player ID. Optional.',
		required: false
	})
	id?: number;

	@ApiProperty({
		description: 'Player username. Optional.',
		required: false
	})
	name?: string;
}

