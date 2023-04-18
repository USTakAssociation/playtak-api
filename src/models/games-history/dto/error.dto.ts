/* eslint-disable indent */
import { ApiProperty } from '@nestjs/swagger';

export class DefaultExceptionDto {
	@ApiProperty()
	statusCode: number;

	@ApiProperty()
	message: string;

	@ApiProperty()
	error: string;
}
