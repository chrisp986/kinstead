import { expect, test } from '@playwright/test';

test('manages work and purchases a shipment from the household dashboard', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1, name: 'Bjornvik' })).toBeVisible();
	await expect(page.getByText('150', { exact: true })).toBeVisible();
	await expect(page.getByText('30 Provisions')).toBeVisible();
	await expect(page.getByRole('heading', { name: 'What happened' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Contracts' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Relationships' })).toBeVisible();
	await expect(page.getByText('+2 trust', { exact: true })).toBeVisible();
	await expect(page.getByText('Due tick 20 · arrived tick 21', { exact: true })).toBeVisible();
	await expect(page.getByText('-8 trust', { exact: true })).toBeVisible();
	await expect(page.getByText('Bjorn completed agriculture.')).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Jarl demands' })).toBeVisible();
	await expect(page.getByText(/18\s+wood → \+10 standing/)).toBeVisible();
	await expect(page.getByText(/6\s+silver → \+10 standing/)).toBeVisible();
	await expect(page.getByText(/13\s+wood → \+7 standing/)).toBeVisible();
	await expect(page.getByText('Serve for 6 ticks · +7 standing').first()).toBeVisible();
	const laborDemands = page.locator('article.demand').filter({ hasText: 'Labor service' });
	await laborDemands.nth(0).getByRole('button', { name: 'Refuse', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('Jarl demand resolved');
	const unavailableLabor = page
		.locator('article.demand')
		.filter({ hasText: 'No household member is available for service.' });
	await expect(unavailableLabor.getByRole('button', { name: 'Serve', exact: true })).toBeDisabled();
	await expect(unavailableLabor.getByRole('button', { name: 'Refuse', exact: true })).toBeEnabled();
	await unavailableLabor.getByRole('button', { name: 'Refuse', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('Jarl demand resolved');

	await page.getByRole('button', { name: 'Accept promise' }).click();
	await expect(page.getByRole('status')).toContainText('Contract accepted');
	await expect(page.getByText('Due tick 2', { exact: true })).toBeVisible();
	await page.getByRole('button', { name: 'Dispatch goods' }).click();
	await expect(page.getByRole('status')).toContainText('Shipment dispatched for arrival at tick 2');
	await expect(page.getByText('Dispatched', { exact: true })).toBeVisible();

	await page.getByLabel('Household member').selectOption({ label: 'Astrid' });
	await page.getByLabel('Activity').selectOption('fishing');
	await page.getByRole('button', { name: 'Schedule work' }).click();
	await expect(page.getByRole('status')).toContainText("Astrid's work was scheduled");
	await expect(page.getByText('Fishing, ticks 1–3')).toBeVisible();
	await expect(page.getByText('Astrid was assigned to fishing for ticks 1–3.')).toBeVisible();

	await page.getByRole('button', { name: 'Buy for delivery' }).click();
	await expect(page.getByRole('status')).toContainText('Shipment is expected at tick 2');
	await expect(page.getByText('5 Provisions', { exact: true })).toBeVisible();
	await expect(page.getByText('In transit').first()).toBeVisible();
	await expect(page.getByText('Bought 5 provisions from Hrafnstead for 7.5 silver.')).toBeVisible();
});
