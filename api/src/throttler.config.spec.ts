import { DEFAULT_THROTTLE_LIMIT, DEFAULT_THROTTLE_TTL_MS, throttlerConfig } from './throttler.config';

describe('throttlerConfig', () => {
	it('defaults to a one minute window, expressed in milliseconds', () => {
		expect(throttlerConfig({})).toEqual([{ ttl: 60000, limit: 60 }]);
		expect(DEFAULT_THROTTLE_TTL_MS).toEqual(60000);
		expect(DEFAULT_THROTTLE_LIMIT).toEqual(60);
	});

	it('reads overrides from the environment', () => {
		expect(throttlerConfig({ THROTTLE_TTL_MS: '30000', THROTTLE_LIMIT: '10' })).toEqual([
			{ ttl: 30000, limit: 10 }
		]);
	});

	it('falls back to the defaults for values that are not positive numbers', () => {
		expect(throttlerConfig({ THROTTLE_TTL_MS: 'soon', THROTTLE_LIMIT: '0' })).toEqual([{ ttl: 60000, limit: 60 }]);
	});
});
