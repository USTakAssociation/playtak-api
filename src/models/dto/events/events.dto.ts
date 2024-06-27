export class Event {
	category: string;
	details: string;
	end_date: string;
	event: string;
	name: string;
	registration: string | null;
	start_date: string;
}

export class EventList {
	categories: Array<string>;
	data: Array<Event>;
}