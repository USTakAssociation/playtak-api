// app controller
import { Controller, Get } from '@nestjs/common';


@Controller()
export class AppController {
	constructor() {}

	@Get('health')
	health(): any {
		return {
			status: 'ok',
			timestamp: new Date().toISOString(),
		}
	}
}