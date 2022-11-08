import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import {
	FastifyAdapter,
	NestFastifyApplication,
} from '@nestjs/platform-fastify';
import { fastifyHelmet } from '@fastify/helmet';

const port = process.env.PORT || 3000;

async function bootstrap() {
	const app = await NestFactory.create<NestFastifyApplication>(
		AppModule,
		new FastifyAdapter(),
	);
	app.enableCors({
		origin: process.env.CORS_DOMAIN,
	});
	await app.register(fastifyHelmet);
	await app.listen(port, '0.0.0.0');
}
bootstrap();
