import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
} from 'typeorm';

@Entity()
export class Games {
	@PrimaryGeneratedColumn()
		id: number;

	@Column()
		date: Date;
	
	@Column()
		size: number;
	
	@Column()
		player_white: string;
	
	@Column()
		player_black: string;
	
	@Column()
		notation: string;
	
	@Column()
		result: string;
	
	@Column()
		timertime: number;
	
	@Column()
		timerinc: number;
	
	@Column()
		rating_white: number;
	
	@Column()
		rating_black: number;
	
	@Column()
		unrated: number;
	
	@Column()
		tournament: number;
	
	@Column()
		komi: number;
	
	@Column()
		pieces: number;
	
	@Column()
		capstones: number;
	
	@Column()
		rating_change_white: number;
	
	@Column()
		rating_change_black: number;
}
