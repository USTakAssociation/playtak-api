
import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
	ManyToOne,
	Relation,
} from 'typeorm';
import { Matchup } from './matchup.entity';
import { GameRules } from './gameRules.entity';


@Entity()
export class Game {

	@PrimaryGeneratedColumn()
		id: number;

	@ManyToOne(() => Matchup, matchup => matchup.games)
		matchup: Relation<Matchup>

	@Column({ default: true })
		player1goesFirst: boolean;

	@ManyToOne(() => GameRules)
		rules: Relation<GameRules>;

	/** Filled in once the game was played */
	@Column({ nullable: true, default: null })
		playtakGameId?: number;

	/** Filled in once the game was played */
	@Column({ nullable: true, default: null })
		result?: string;
}
