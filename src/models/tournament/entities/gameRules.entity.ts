
import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
} from 'typeorm';


@Entity()
export class GameRules {
	@PrimaryGeneratedColumn()
		id: number;

	@Column()
		name: string;

	/** Seconds */
	@Column()
		timeContingent: number;

	/** Seconds */
	@Column()
		timeIncrement: number;

	@Column()
		extraTimeTriggerMove: number;

	/** Seconds */
	@Column()
		extraTimeAmount: number;

	@Column()
		komi: number;

	@Column()
		boardSize: number;

	@Column()
		capstones: number;

	@Column()
		pieces: number;
}
