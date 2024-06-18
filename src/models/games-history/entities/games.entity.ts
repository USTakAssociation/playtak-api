import {
	Entity,
	Column,
	PrimaryGeneratedColumn
} from 'typeorm';
@Entity()
export class Games {
	@PrimaryGeneratedColumn()
		id: number;

	@Column()
		date: number;

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

	@Column({ default: 0 })
		timertime: number;

	@Column({ default: 0 })
		timerinc: number;

	@Column({ default: 1000 })
		rating_white: number;

	@Column({ default: 1000 })
		rating_black: number;

	@Column({ default: 0 })
		unrated: number;

	@Column({ default: 0 })
		tournament: number;

	@Column({ default: 0 })
		komi: number;

	@Column({ default: -1 })
		pieces: number;

	@Column({ default: -1 })
		capstones: number;

	@Column({ default: 0 })
		rating_change_white: number;

	@Column({ default: 0 })
		rating_change_black: number;
}
