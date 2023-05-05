import { ApiProperty } from '@nestjs/swagger';
import { IsBoolean, IsInt } from 'class-validator';
import { GameRulesDto } from './gameRules.dto';
import { MatchupDto } from './matchups.dto';

/** Filters applied when getting all games */
export class GameQuery {

}

class BaseGameDto {
	@IsBoolean()
	@ApiProperty()
		player1goesFirst: boolean;
}

export class CreateGameDto extends BaseGameDto{
	@IsInt()
	@ApiProperty()
		matchup: number

	@IsInt()
	@ApiProperty()
		rules: number
}

export class GameDto extends BaseGameDto{
	@ApiProperty()
		id: number;

	@ApiProperty()
		matchup: MatchupDto|number

	@ApiProperty()
		rules: GameRulesDto|number

	@ApiProperty()
		playtakId?: number;

	@ApiProperty()
		result?: string;
}
