import { expect, test } from '@playwright/test';

const adminToken = 'A'.repeat(43);
const ordinaryToken = 'P'.repeat(43);
const householdId = '00000000-0000-0000-0000-000000000020';
const playerId = '00000000-0000-0000-0000-000000000010';

test('ordinary players cannot open the developer console', async ({ page, context }) => {
	await context.addCookies([
		{ name: 'game_session', value: ordinaryToken, url: 'http://127.0.0.1:4173' }
	]);
	const response = await page.goto('/admin');
	await expect(page.getByText('Developer-console access is required.')).toBeVisible();
	expect(response?.status()).toBe(403);
});

test.describe('administrator console', () => {
	test.beforeEach(async ({ context }) => {
		await context.addCookies([
			{ name: 'game_session', value: adminToken, url: 'http://127.0.0.1:4173' }
		]);
	});

	test('inspects an unowned household and shows explicit empty states', async ({ page }) => {
		await page.goto(`/admin/households/${householdId}`);
		await expect(page.getByRole('heading', { name: 'Bjornvik' })).toBeVisible();
		await expect(page.getByText('No characters in this snapshot.')).toBeVisible();
		await expect(page.getByText('No tick diagnostics recorded.')).toBeVisible();
		await expect(page.getByText('None recorded.').first()).toBeVisible();
		await expect(page.locator('body')).not.toContainText(/token_hash|game_session|authorization/i);
		await expect(page.locator('button')).toHaveCount(0);
	});

	test('distinguishes waiting and stale-worker world health', async ({ page }) => {
		await page.goto('/admin');
		await expect(page.getByRole('link', { name: 'Bjornvik world' })).toBeVisible();
		await expect(page.getByText('waiting', { exact: true })).toBeVisible();
		await page.getByRole('link', { name: 'Stalled world' }).click();
		await expect(page.getByText('worker unavailable', { exact: true })).toBeVisible();
		await expect(page.getByText('no heartbeat within 30 seconds')).toBeVisible();
	});

	test('renders safe account and empty session history', async ({ page }) => {
		await page.goto(`/admin/accounts/${playerId}`);
		await expect(page.getByRole('heading', { name: playerId })).toBeVisible();
		await expect(page.getByText('No sessions recorded.')).toBeVisible();
		await expect(page.locator('body')).not.toContainText(/token_hash|game_session|authorization/i);
	});
});
