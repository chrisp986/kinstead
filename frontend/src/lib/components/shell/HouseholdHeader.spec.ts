import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { HouseholdReport } from '$lib/api/generated';
import HouseholdHeader from './HouseholdHeader.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function mattersMarkup(
	attention: HouseholdReport['attention'],
	decisions: HouseholdReport['decisions']
) {
	const report = {
		household_id: 'household',
		household_name: 'Hof Stenby',
		setting_start_year: 980,
		calendar: { game_day: 0, year_index: 0, production_season: 'spring', week_of_half: 1 },
		characters: [],
		assignments: [],
		supply_game_days: 31,
		supply_status: 'safe',
		attention,
		decisions
	} as unknown as HouseholdReport;
	return render(HouseholdHeader, { props: { report } }).body;
}

describe('household matter count', () => {
	it('counts the same issue in attention and decisions once', () => {
		const item = {
			code: 'food_supply_critical',
			related_id: 'food'
		} as HouseholdReport['attention'][number];
		expect(mattersMarkup([item], [item])).toMatch(/Matters<\/span>\s*<strong[^>]*>1<\/strong>/);
	});

	it('keeps different related issues separate and normalizes missing IDs', () => {
		const item = {
			code: 'fatigue_warning',
			related_id: 'one'
		} as HouseholdReport['attention'][number];
		expect(mattersMarkup([item], [{ ...item, related_id: 'two' }])).toMatch(
			/Matters<\/span>\s*<strong[^>]*>2<\/strong>/
		);
		const noID = { code: 'food_supply_critical' } as HouseholdReport['attention'][number];
		expect(mattersMarkup([noID], [{ ...noID, related_id: '' }])).toMatch(
			/Matters<\/span>\s*<strong[^>]*>1<\/strong>/
		);
	});
});
