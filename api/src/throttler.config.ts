import { ThrottlerModuleOptions } from '@nestjs/throttler';

// `ttl` is milliseconds (node_modules/@nestjs/throttler/dist/throttler.service.js:49).
// The per-module registrations this replaced used `ttl: 60`, i.e. a 60ms window, which
// left rate limiting effectively off.
export const DEFAULT_THROTTLE_TTL_MS = 60000;
export const DEFAULT_THROTTLE_LIMIT = 60;

export function throttlerConfig(env: NodeJS.ProcessEnv = process.env): ThrottlerModuleOptions {
	return [
		{
			ttl: Number(env.THROTTLE_TTL_MS) || DEFAULT_THROTTLE_TTL_MS,
			limit: Number(env.THROTTLE_LIMIT) || DEFAULT_THROTTLE_LIMIT
		}
	];
}
