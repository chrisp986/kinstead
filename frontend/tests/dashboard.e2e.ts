import { expect, test } from '@playwright/test';

async function expectMobileNavContent(page: import('@playwright/test').Page) {
	const nav = page.getByRole('navigation', { name: 'Household sections' });
	await expect(nav).toBeVisible();
	for (const label of ['Report', 'Calendar', 'Farm', 'Work', 'Trade', 'Chronicle']) {
		const link = nav.getByRole('link', { name: label, exact: true });
		await expect(link).toBeVisible();
		const generated = await link.evaluate((element) => ({
			icon: getComputedStyle(element, '::before').content,
			label: getComputedStyle(element, '::after').content,
			color: getComputedStyle(element).color,
			opacity: getComputedStyle(element).opacity,
			visibility: getComputedStyle(element).visibility
		}));
		expect(generated.icon).not.toBe('none');
		expect(generated.icon).not.toBe('""');
		expect(generated.label).toContain(label);
		expect(generated.color).not.toBe('rgba(0, 0, 0, 0)');
		expect(generated.opacity).toBe('1');
		expect(generated.visibility).toBe('visible');
	}
	return nav;
}

test('keeps the five household surfaces connected on mobile', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1, name: 'Bjornvik' })).toBeVisible();
	await expect(page.getByText('980 CE · spring · day 1 of 31')).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Household report' }).first()).toBeVisible();
	await expect(page.getByRole('heading', { name: 'What needs your decision?' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Since you were away' })).toBeVisible();
	await expect(page.getByText(/suffered a food shortage/i)).toBeVisible();
	await expectMobileNavContent(page);

	const provisions = page.getByLabel('Household status').getByRole('link').first();
	await expect(provisions).toContainText('30 days');
	await expect(provisions).not.toContainText(/\d+\.\d+ days/);

	// Flow A: report → persistent occupation preview/change → report.
	await page.getByRole('link', { name: 'Work', exact: true }).click();
	await expect(page).toHaveURL(/\/work$/);
	await expect(page.getByText('Today’s work: 09:00–17:00')).toBeVisible();
	await expect(page.getByText('Regular occupation').first()).toBeVisible();
	await page.getByLabel('Character').selectOption({ label: 'Astrid' });
	await page.getByLabel('Occupation').selectOption('agriculture');
	await expect(page.getByText('Proposed after 7 days')).toBeVisible();
	await page.getByRole('button', { name: 'Change occupation' }).click();
	await expect(page.getByRole('status')).toContainText('Occupation plan updated');
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
	const quoteQuantity = page.getByLabel('Quantity').first();
	await quoteQuantity.fill('5');
	await expect(page.getByText('Silver after purchase')).toBeVisible();
	await expect(page.getByText('Expected arrival', { exact: true })).toBeVisible();
	await expect(page.getByText('Hrafnstead').first()).toBeVisible();
	await page.getByRole('button', { name: 'Buy for delivery' }).click();
	await expect(page.getByRole('status')).toContainText('shipment is on its way');
	await expect(page.getByRole('heading', { name: 'Journeys' })).toBeVisible();
	await expect(page.getByText('Incoming · Hrafnstead').first()).toBeVisible();

	// Flow D: accept and dispatch a contract, then inspect Transit.
	await page.getByRole('button', { name: /Propose recurring delivery/ }).click();
	await page.getByLabel('Units each time').fill('6');
	await expect(page.getByText(/deliveries.*total/i)).toBeVisible();
	await expect(page.getByText(/one-way delivery obligation/i)).toBeVisible();
	await page.getByRole('button', { name: 'Send proposal' }).click();
	await expect(page.getByRole('status')).toContainText('Contract proposal sent');
	await page.getByRole('button', { name: 'Accept promise' }).click();
	await expect(page.getByRole('status')).toContainText('Contract accepted');
	await expect(page.getByText('Next delivery')).toBeVisible();
	await expect(page.locator('body')).not.toContainText(/Household …[0-9a-f]{6}/i);

	// The accepted schedule is projected into both calendar action and due events.
	await page.getByRole('link', { name: 'Calendar', exact: true }).click();
	await expect(page).toHaveURL(/\/calendar$/);
	await expect(page.getByText('Dispatch shipment')).toBeVisible();
	await expect(page.getByText('Delivery due')).toBeVisible();
	await page.getByRole('button', { name: 'Contracts', exact: true }).click();
	await expect(page.getByText('Delivery due')).toBeVisible();
	await expect(page.getByText('Midsummer')).toHaveCount(0);
	await page.getByRole('button', { name: 'All', exact: true }).click();
	await expect(page.locator('body')).not.toContainText(/00000000-0000-0000-0000-000000000[0-9]{3}/);
	await expect(page.getByRole('tab', { name: 'Year cycle' })).toHaveCount(0);
	await expect(page.getByText(/days until the next season/i).first()).toBeVisible();
	await expect(page.locator('body')).not.toContainText(/(?:game_day|current_tick|\btick\b)/i);
	await page.getByRole('link', { name: 'Trade', exact: true }).click();
	await page.getByRole('button', { name: 'Dispatch goods' }).click();
	await expect(page.getByRole('status')).toContainText('Shipment dispatched. It is on the way.');
	await expect(page.getByText('In transit').first()).toBeVisible();
	await page.getByRole('link', { name: 'Calendar', exact: true }).click();
	await expect(page.getByText('Shipment expected')).toBeVisible();
	await expect(page.getByText('Dispatch shipment')).toHaveCount(0);

	await page.getByRole('link', { name: 'Chronicle', exact: true }).click();
	await expect(page).toHaveURL(/\/chronicle$/);
	await expect(page.getByRole('heading', { name: 'Chronicle' })).toBeVisible();
	await expect(page.getByText('Work completed')).toBeVisible();
});

test('mobile navigation leaves room for the final controls', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 568 });
	await page.goto('/households/00000000-0000-0000-0000-000000000020/work');
	const nav = await expectMobileNavContent(page);
	const button = page.getByRole('button', { name: 'Change occupation' });
	await expect(button).toBeVisible();
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
		{ width: 375, height: 812 },
		{ width: 390, height: 844 },
		{ width: 430, height: 932 },
		{ width: 768, height: 1024 },
		{ width: 1280, height: 800 },
		{ width: 1440, height: 900 }
	]) {
		await page.setViewportSize(viewport);
		for (const suffix of ['', '/farm', '/work', '/trade', '/calendar', '/chronicle']) {
			await page.goto(`/households/00000000-0000-0000-0000-000000000020${suffix}`);
			const overflow = await page.evaluate(
				() => document.documentElement.scrollWidth > window.innerWidth + 1
			);
			expect(overflow, `horizontal overflow at ${viewport.width}x${viewport.height}${suffix}`).toBe(
				false
			);
			await expect(page.locator('body')).not.toContainText(/(?:…|\.{3})[0-9a-f]{6}\b/i);
			await expect(page.locator('body')).not.toContainText(
				/\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/i
			);
		}
	}
});

test('shared household header advances the historical year after rollover', async ({ page }) => {
	await page.goto('/households/00000000-0000-0000-0000-000000000022');
	await expect(page.getByText('981 CE · Spring · first week')).toBeVisible();
});
