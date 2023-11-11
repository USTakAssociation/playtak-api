import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import {
	FastifyAdapter,
	NestFastifyApplication,
} from '@nestjs/platform-fastify';
import { fastifyHelmet } from '@fastify/helmet';
import { VersioningType } from '@nestjs/common';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';
import * as path from 'path';
import { writeFileSync } from 'fs';

const port = process.env.PORT || 3000;

async function bootstrap() {
	const app = await NestFactory.create<NestFastifyApplication>(
		AppModule,
		new FastifyAdapter(),
	);
	app.enableVersioning({
		type: VersioningType.URI,
	});
	
	const config = new DocumentBuilder()
		.addApiKey({
			type: 'apiKey',
			name: 'Ocp-Apim-Subscription-Key',
			in: 'header',
		})
		.setTitle(`Play Tak API Service ${process.env.APP_ENV}`)
		.setDescription('Play Tak API Service Swagger API documentation')
		.setVersion('1.0')
		.addTag(`Play Tak API Service ${process.env.APP_ENV}`)
		.build();
	const document = SwaggerModule.createDocument(app, config);
	SwaggerModule.setup('api', app, document);
	let dist = '';
	if (process.env.DIST_PATH) {
		dist = process.env.DIST_PATH;
	}
	const outputPath = path.resolve(process.cwd(), path.join(dist, 'swagger.json'));
	writeFileSync(outputPath, JSON.stringify(document), { encoding: 'utf8' });

	app.enableCors({
		origin: process.env.CORS_DOMAIN.split(','),
	});

	await app.register(fastifyHelmet, {
		contentSecurityPolicy: false, 
	});
	await app.listen(port, '0.0.0.0');
}
bootstrap();
