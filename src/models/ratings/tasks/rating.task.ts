import { Injectable, Logger } from '@nestjs/common';
import { Cron } from '@nestjs/schedule';
import { RatingService } from '../ratings.service';


@Injectable()
export class RatingTask {
	private readonly logger = new Logger(RatingTask.name);

	constructor(private _service: RatingService) {}

	// * * * * * *
	// | | | | | |
	// | | | | | day of week
	// | | | | months
	// | | | day of month
	// | | hours
	// | minutes
	// seconds (optional)
	@Cron('0 15 * * * *')
	handleCron() {
		this.logger.debug('Running rating task');
		this._service.generateRating();
	}
}
