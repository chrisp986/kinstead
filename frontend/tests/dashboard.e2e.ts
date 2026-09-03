import { expect, test } from '@playwright/test';

test('manages work and purchases a shipment from the household dashboard', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1, name: 'Bjornvik' })).toBeVisible();
	await expect(page.getByText('150', { exact: true })).toBeVisible();
	await expect(page.getByText('30 Provisions')).toBeVisible();
	await expect(page.getByRole('heading', { name: 'What happened' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Contracts' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Relationships' })).toBeVisible();
	await expect(page.getByText('Bjorn completed agriculture.')).toBeVisible();

	await page.getByRole('button', { name: 'Accept promise' }).click();
	await expect(page.getByRole('status')).toContainText('Contract accepted');
	await expect(page.getByText('Due tick 2')).toBeVisible();
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
