import { ApiProperty } from '@nestjs/swagger';
import { IsInt, IsString } from 'class-validator';
import { GameDto } from './game.dto';
import { GroupDto } from './group.dto';

class BaseMatchupDto {
	@IsString()
	@ApiProperty()
		player1: string

	@IsString()
	@ApiProperty()
		player2: string
}

export class CreateMatchupDto extends BaseMatchupDto{
	@IsInt()
	@ApiProperty()
		group: number

}

export class MatchupDto extends BaseMatchupDto{
	@ApiProperty()
		id: number

	@ApiProperty()
		group: GroupDto|number

	@ApiProperty()
		games?: Array<GameDto>
}
