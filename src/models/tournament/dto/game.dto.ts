import { ApiProperty } from '@nestjs/swagger';
import { IsBoolean, IsInt } from 'class-validator';
import { GameRulesDto } from './gameRules.dto';
import { MatchupDto } from './matchups.dto';

/** Filters applied when getting all games */
export class GameQuery {

}

export class CreateGameDto {
	@IsInt()
	@ApiProperty()
		matchup: number
	
	@IsBoolean()
	@ApiProperty()
		player1goesFirst: boolean;

	@IsInt()
	@ApiProperty()
		rules: number
}

export class GameDto {
	@ApiProperty()
		id: number;

	@ApiProperty()
		matchup?: MatchupDto

	@ApiProperty()
		player1goesFirst: boolean;

	@ApiProperty()
		rules: GameRulesDto

	@ApiProperty()
		playtakId?: number;

	@ApiProperty()
		result?: string;
}
