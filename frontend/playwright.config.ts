import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './tests',
	testMatch: '**/*.e2e.{ts,js}',
	use: { baseURL: 'http://127.0.0.1:4173' },
	webServer: [
		{ command: 'node tests/mock-api.mjs', port: 9080, reuseExistingServer: false },
		{
			command: 'npm run build && npm run preview -- --host 127.0.0.1',
			port: 4173,
			env: { BACKEND_URL: 'http://127.0.0.1:9080' },
			reuseExistingServer: false
		}
	]
});
