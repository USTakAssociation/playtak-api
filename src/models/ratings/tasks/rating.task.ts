import { Injectable, Logger } from '@nestjs/common';
import { Cron } from '@nestjs/schedule';
import { RatingService } from '../ratings.service';


@Injectable()
export class RatingTask {
	private readonly logger = new Logger(RatingTask.name);

	constructor(private _service: RatingService) {}

	// TODO enable this when ready to move awai rom rating script
	//@Cron('30 */1 * * *')
	//@Cron('* * * * *')
	handleCron() {
		this.logger.debug('Running rating task');
		this._service.generateRating();
	}
}
