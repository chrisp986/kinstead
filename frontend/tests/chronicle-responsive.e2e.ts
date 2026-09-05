import { expect, test } from '@playwright/test';

test('Chronicle timeline stacks dates above events on narrow screens', async ({ page }) => {
	await page.setViewportSize({ width: 375, height: 812 });
	await page.goto('/households/00000000-0000-0000-0000-000000000020/chronicle');

	await expect(page.getByRole('heading', { name: 'What happened' })).toBeVisible();
	const firstGroup = page.locator('.day-group').first();
	const marker = firstGroup.locator('.day-marker');
	const events = firstGroup.locator('.day-events');

	const markerBox = await marker.boundingBox();
	const eventsBox = await events.boundingBox();
	expect(markerBox).not.toBeNull();
	expect(eventsBox).not.toBeNull();
	if (markerBox && eventsBox) {
		expect(markerBox.width).toBeGreaterThan(250);
		expect(eventsBox.y).toBeGreaterThanOrEqual(markerBox.y + markerBox.height - 1);
	}

	const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1);
	expect(overflow).toBe(false);
});
