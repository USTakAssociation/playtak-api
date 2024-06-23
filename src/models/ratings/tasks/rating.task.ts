import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { SchedulerRegistry } from '@nestjs/schedule';
import { RatingService } from '../ratings.service';
import { CronJob } from 'cron';


@Injectable()
export class RatingTask implements OnModuleInit {
	private readonly logger = new Logger(RatingTask.name);

	constructor(
		private _service: RatingService,
		private schedulerRegistry: SchedulerRegistry
	) {}

	onModuleInit() {
		this.logger.log('Rating task initialized');
		if (process.env.RATING_CRON_VALUE) {
			const job = new CronJob(process.env.RATING_CRON_VALUE, () => {
				// What you want to do here
				this.handleTask();
			});
			
			this.schedulerRegistry.addCronJob('RatingTask', job);
			job.start();
		}
	}
	// * * * * * *
	// | | | | | |
	// | | | | | day of week
	// | | | | months
	// | | | day of month
	// | | hours
	// | minutes
	// seconds (optional)
	//@Cron('0 15 * * * *')
	handleTask() {
		this.logger.debug('Running rating task');
		this._service.generateRating();
	}
}
