/* eslint-disable indent */
import { ApiProperty } from '@nestjs/swagger';

export class DefaultReposeDto {
	@ApiProperty()
	status: number;
	
	@ApiProperty()
	message: string;
}