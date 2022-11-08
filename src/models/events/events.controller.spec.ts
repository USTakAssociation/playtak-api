import { CacheModule } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { EventsController } from './events.controller';
import { EventsService } from './events.service';

describe('EventsController', () => {
	let controller: EventsController;

	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			imports: [CacheModule.register()],
			controllers: [EventsController],
			providers: [EventsService]
		}).compile();

		controller = module.get<EventsController>(EventsController);
	});

	it('should be defined', () => {
		expect(controller).toBeDefined();
	});
});
