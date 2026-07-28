import { Entity, Column, PrimaryGeneratedColumn, Index } from 'typeorm';
@Entity({
	name: 'players'
})
@Index(['name'])
export class Ratings {
	@PrimaryGeneratedColumn()
	id: number;

	@Column()
	name: string;

	@Column()
	rating: number;

	@Column()
	maxrating: number;

	@Column()
	ratedgames: number;

	@Column()
	isbot: boolean;

	@Column()
	ratingbase?: number;

	@Column()
	participation_rating: number;

	@Column()
	unrated?: number;
}
