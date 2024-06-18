import { HttpException, Injectable, Logger, NotFoundException } from '@nestjs/common';
import { google } from 'googleapis';
import * as path from 'path';
import { EventList } from '../games-history/dto/events.dto';

@Injectable()
export class EventsService {
	private filePath;
	private CREDENTIALS_PATH;
	private SCOPES = ['https://www.googleapis.com/auth/spreadsheets'];
	private readonly logger = new Logger(EventsService.name);

	constructor() {
		this.filePath = path.resolve(__dirname, '../../assets');
		this.CREDENTIALS_PATH = path.join(this.filePath, '/client_secret.json');
	}

	async getEvents(): Promise<EventList | NotFoundException | HttpException> {
		try {
			const auth = new google.auth.GoogleAuth({
				keyFile: this.CREDENTIALS_PATH,
				scopes: this.SCOPES,
			});
			const sheets = google.sheets({ version: 'v4', auth });
			const res = sheets.spreadsheets.values.get({
				spreadsheetId: '1kpgv_7pkxijsjpQx5iHZAF3jnsZLlBabkKh49pAHhu8',
				range: 'Event List!A2:H',
			});
			const response = (await res).data.values;
			if (!response || !response.length) {
				return new NotFoundException({
					message: 'No data found',
				});
			}
			const data = [];
			const categories = ['All'];

			for (let i = 0; i < response.length; i++) {
				const tempObject = {
					name: response[i][0] || '',
					event: response[i][1] || '',
					category: response[i][2] || '',
					start_date: response[i][3] || null,
					end_date: response[i][4] || null,
					details:
						response[i][5] && response[i][5].startsWith('http')
							? response[i][5]
							: null,
					registration:
						response[i][6] && response[i][6].startsWith('http')
							? response[i][6]
							: null,
					standings: 
						response[i][7] && response[i][7].startsWith('http') 
							? response[i][7] 
							: null,
				};
				categories.push(response[i][2]);
				data.push(tempObject);
			}
			return {
				data,
				categories: [...new Set(categories)],
			};
		} catch (error) {
			this.logger.error(error);
			return new HttpException('500', error);
		}
	}
}
