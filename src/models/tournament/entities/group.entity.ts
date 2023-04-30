
import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
	ManyToOne,
	OneToMany,
	Relation,
} from 'typeorm';
import { Matchup } from './matchup.entity';
import { Stage } from './stage.entity';


@Entity()
export class Group {
	@PrimaryGeneratedColumn()
		id: number

	@ManyToOne(() => Stage, stage => stage.groups)
		stage: Relation<Stage>
	
	@Column({ nullable: true, default: null })
		name?: string
	
	@OneToMany(() => Matchup, matchup => matchup.group)
		matchups: [Relation<Matchup>]
}
