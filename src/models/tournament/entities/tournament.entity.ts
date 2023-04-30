
import {
	Entity,
	Column,
	PrimaryGeneratedColumn,
	OneToMany,
	Relation,
} from 'typeorm';
import { Stage } from './stage.entity';


@Entity()
export class Tournament { // part of mvp
	@PrimaryGeneratedColumn()
		id: number
	@Column()
		name: string
	@Column()
		description: string
	/** `true` if the tournament is over. */
	@Column({ default: false })
		finished: boolean
	// @Column()
	// 	type: string // tbd
	// @Column()
	// matchupType: "single game" | "double game" // tbd

	@OneToMany(() => Stage, stage => stage.tournament)
		stages: [Relation<Stage>]

	// maybe skip these for now
	// applications?: any
	// acceptedApplications?: any
	// entryRequirements?: TournamentPlayerEntryRequirements
}
