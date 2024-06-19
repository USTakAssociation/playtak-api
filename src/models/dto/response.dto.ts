/* eslint-disable indent */
import { ApiProperty } from '@nestjs/swagger';

export class DefaultResponseDto {
	@ApiProperty()
	status: number;
	
	@ApiProperty()
	message: string;
}