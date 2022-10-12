import { Injectable, NotFoundException } from '@nestjs/common';
import { google } from 'googleapis';
import * as path from 'path';

@Injectable()
export class EventsService {
	private filePath;
	private CREDENTIALS_PATH;
	private SCOPES = ['https://www.googleapis.com/auth/spreadsheets'];
	constructor() {
		this.filePath = path.resolve(__dirname, '../../assets');
		this.CREDENTIALS_PATH = this.filePath + '/client_secret.json';
	}

	async getEvents() {
		try {
			const auth = new google.auth.GoogleAuth({
				keyFile: this.CREDENTIALS_PATH,
				scopes: this.SCOPES,
			});
			const sheets = google.sheets({version: 'v4', auth})
			const res = sheets.spreadsheets.values.get({
				spreadsheetId: '1kpgv_7pkxijsjpQx5iHZAF3jnsZLlBabkKh49pAHhu8',
				range: 'Event List!A2:F'
			});
			const response = (await res).data.values;
			if(!response || !response.length){
				return new NotFoundException({
					message: 'No data found'
				})
			}
			const data = [];
			const categories = ['All']
			for (let i = 0; i < response.length; i++) {
				const tempObject = {
					name: response[i][0] || "",
					event: response[i][1] || "",
					category: response[i][2] || "",
					start_date: response[i][3] || "",
					end_date: response[i][4] || "",
					link: response[i][5] || null,
				};
				categories.push(response[i][2]);
				data.push(tempObject)
			}
			return {
				data,
				categories: [...new Set(categories)]
			};
		} catch (error) {
			console.error(error);
		}
	}
}
