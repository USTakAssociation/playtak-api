import { ApiProperty } from '@nestjs/swagger';
import { IsInstance, IsInt, IsOptional, IsString } from 'class-validator';
import { GameDto } from './game.dto';


export class MatchupsQuery {

}

export class MatchupDto {
	@ApiProperty()
		id: number

	@ApiProperty()
		group: any // TODO

	@ApiProperty()
		player1: string

	@ApiProperty()
		player2: string

	@ApiProperty()
		games: Array<GameDto>
}

export class CreateMatchupDto {
	@IsInt()
	@ApiProperty()
		group?: number

	@IsString()
	@ApiProperty()
		player1: string

	@IsString()
	@ApiProperty()
		player2: string

	@IsOptional()
	@IsInstance(Array<GameDto>)
	@ApiProperty()
		games?: Array<GameDto>
}