import { Entity, Column, PrimaryGeneratedColumn } from 'typeorm';
@Entity()
export class Players {
	@PrimaryGeneratedColumn()
		id: number;

	@Column()
		name: string;

	@Column()
		password: string;

	@Column()
		email: string;
		
	@Column()
		rating: number;

	@Column()
		boost: number;

	@Column()
		maxrating: number;

	@Column()
		ratedgames: number;

	@Column()
		isbot: boolean;
	
	@Column()
		ratinggage: number;
		
	@Column()
		ratingbase: number;
		
	@Column()
		unrated: boolean
		
	@Column()
		fatigue: string
		
	@Column()
		is_admin: boolean
		
	@Column()
		is_mod: boolean
		
	@Column()
		primary_account: number
}
