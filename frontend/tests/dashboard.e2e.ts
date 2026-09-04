import { expect, test } from '@playwright/test';

test('keeps the five household surfaces connected on mobile', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1, name: 'Bjornvik' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Farm report' }).first()).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Decide now' })).toBeVisible();
	await expect(page.getByRole('navigation', { name: 'Household sections' })).toBeVisible();

	// Flow A: report → work → schedule → report.
	await page.getByRole('link', { name: 'Work' }).click();
	await expect(page).toHaveURL(/\/work$/);
	await page.getByLabel('Who?').selectOption({ label: 'Astrid' });
	await page.getByLabel('Activity').selectOption('fishing');
	await page.getByRole('button', { name: 'Schedule work' }).click();
	await expect(page.getByRole('status')).toContainText("Astrid's work was scheduled");
	await page.getByRole('link', { name: 'Report' }).click();
	await expect(page).toHaveURL(/\/households\/[^/]+$/);

	// Flow B: report decision → Farm politics → response feedback.
	await page.getByRole('link', { name: /Respond to Jarl Eirik/ }).click();
	await expect(page).toHaveURL(/\/farm#politics$/);
	await page.getByRole('button', { name: 'Refuse', exact: true }).first().click();
	await expect(page.getByRole('status')).toContainText('Jarl demand resolved');

	// Flow C: Trade → purchase → transit.
	await page.getByRole('link', { name: 'Trade' }).click();
	await expect(page).toHaveURL(/\/trade$/);
	await page.getByRole('button', { name: 'Buy for delivery' }).click();
	await expect(page.getByRole('status')).toContainText('Shipment is expected at tick 2');
	await expect(page.getByRole('heading', { name: 'Shipments' })).toBeVisible();

	// Flow D: accept and dispatch a contract, then inspect Transit.
	await page.getByRole('button', { name: 'Accept promise' }).click();
	await expect(page.getByRole('status')).toContainText('Contract accepted');
	await page.getByRole('button', { name: 'Dispatch goods' }).click();
	await expect(page.getByRole('status')).toContainText('Shipment dispatched for arrival at tick 2');
	await expect(page.getByText('In transit').first()).toBeVisible();

	await page.getByRole('link', { name: 'Chronicle' }).click();
	await expect(page).toHaveURL(/\/chronicle$/);
	await expect(page.getByRole('heading', { name: 'Chronicle' })).toBeVisible();
	await expect(page.getByText('Work completed')).toBeVisible();
});

test('mobile navigation leaves room for the final controls', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 568 });
	await page.goto('/households/00000000-0000-0000-0000-000000000020/work');
	const nav = page.getByRole('navigation', { name: 'Household sections' });
	await expect(nav).toBeVisible();
	await expect(page.getByRole('button', { name: 'Schedule work' })).toBeVisible();
	const button = page.getByRole('button', { name: 'Schedule work' });
	await button.scrollIntoViewIfNeeded();
	const navBox = await nav.boundingBox();
	const buttonBox = await button.boundingBox();
	expect(navBox).not.toBeNull();
	expect(buttonBox).not.toBeNull();
	if (navBox && buttonBox) expect(buttonBox.y + buttonBox.height).toBeLessThanOrEqual(navBox.y + 1);
});

test('household surfaces fit the supported viewport range', async ({ page }) => {
	for (const viewport of [
		{ width: 320, height: 568 },
		{ width: 375, height: 667 },
		{ width: 390, height: 844 },
		{ width: 430, height: 932 },
		{ width: 768, height: 1024 },
		{ width: 1280, height: 800 },
		{ width: 1440, height: 900 }
	]) {
		await page.setViewportSize(viewport);
		await page.goto('/households/00000000-0000-0000-0000-000000000020');
		await expect(page.getByRole('heading', { name: 'Farm report' }).first()).toBeVisible();
		const overflow = await page.evaluate(
			() => document.documentElement.scrollWidth > window.innerWidth + 1
		);
		expect(overflow, `horizontal overflow at ${viewport.width}x${viewport.height}`).toBe(false);
	}
});
