
import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
	ManyToOne,
	OneToMany,
	Relation,
} from 'typeorm';
import { GameRules } from './gameRules.entity';
import { Group } from './group.entity';
import { Tournament } from './tournament.entity';


@Entity()
export class Stage {
	@PrimaryGeneratedColumn()
		id: number
	
	@ManyToOne(() => Tournament, tournament => tournament.stages)
		tournament: Relation<Tournament>
	
	@Column()
		name: string
	
	@OneToMany(() => Group, group => group.stage)
		groups: Array<Relation<Group>>
	
	@ManyToOne(() => GameRules)
		rules: Relation<GameRules> // tbd: is this necessary? let's have it for now
}

