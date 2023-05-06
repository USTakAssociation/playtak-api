import { ApiProperty } from "@nestjs/swagger";
export class Rating {
	@ApiProperty()
		id: number;
	@ApiProperty()
		name: string;
	@ApiProperty()
		rating: number;
	@ApiProperty()
		maxrating: number;
	@ApiProperty()
		ratedgames: number;
	@ApiProperty()
		isbot: boolean;
}

export class RatingList {
	@ApiProperty()
		items: Array<Rating>;
	@ApiProperty()
		total: number;
	@ApiProperty()
		perPage: number;
	@ApiProperty()
		page: number;
	@ApiProperty()
		totalPages: number;
}

export class RatingQuery {
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
		description: 'player id',
		required: false
	})
		id?: number;
	@ApiProperty({
		description: 'Player username',
		required: false
	})
		name?: string;
}

