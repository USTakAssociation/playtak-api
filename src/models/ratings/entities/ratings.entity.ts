
import { Entity, Column, PrimaryGeneratedColumn } from 'typeorm';
@Entity({
	name: 'players'
})
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
		ratingbase: number;
		
	@Column()
		unrated: number;
}
