
import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
	ManyToOne,
	OneToMany,
	Relation,
} from 'typeorm';
import { Game } from './game.entity';
import { Group } from './group.entity';


@Entity()
export class Matchup {
	@PrimaryGeneratedColumn()
		id: number

	@ManyToOne(() => Group, group => group.matchups)
		group: Relation<Group>

	@Column()
		player1: string // FK

	@Column()
		player2: string // FK

	@OneToMany(() => Game, game => game.matchup)
		games: [Relation<Game>]
}

