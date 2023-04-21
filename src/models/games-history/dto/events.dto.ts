import { ApiProperty } from "@nestjs/swagger";

export class Event {
	@ApiProperty()
		category: string;
	@ApiProperty()
		details: string;
	@ApiProperty()
		end_date: string;
	@ApiProperty()
		event: string;
	@ApiProperty()
		name: string;
	@ApiProperty()
		registration: string | null;
	@ApiProperty()
		start_date: string;
}

export class EventList {
	@ApiProperty()
		categories: Array<string>;
		
	@ApiProperty()
		data: Array<Event>;
}