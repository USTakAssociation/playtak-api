import swc from 'unplugin-swc';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	test: {
		globals: true,
		root: './',
		environment: 'node',
		include: ['src/**/*.spec.ts'],
		coverage: {
			provider: 'v8',
			include: ['src/**/*.{js,ts}'],
			exclude: ['src/**/*.spec.ts'],
			reportsDirectory: './coverage',
			thresholds: {
				branches: 80,
				functions: 20,
				lines: 60,
				statements: 60
			}
		}
	},
	plugins: [
		swc.vite({
			module: { type: 'es6' }
		})
	]
});
