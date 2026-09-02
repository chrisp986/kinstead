import type { ChronicleEntry } from '$lib/api/generated';
import { describe, expect, it } from 'vitest';
import { describeChronicleEntry } from './chronicle';

function entry(values: Partial<ChronicleEntry>): ChronicleEntry {
	return {
		id: '00000000-0000-0000-0000-000000000001',
		occurred_tick: 4,
		entry_type: 'test_event',
		data: {},
		...values
	};
}

describe('chronicle descriptions', () => {
	it('renders structured market and shipment facts', () => {
		expect(
			describeChronicleEntry(
				entry({
					entry_type: 'market_purchase',
					related_household_name: 'Hrafnstead',
					data: { resource_type: 'provisions', quantity_milli: 5_000, cost_milli: 7_500 }
				})
			)
		).toEqual({
			title: 'Market purchase',
			detail: 'Bought 5 provisions from Hrafnstead for 7.5 silver.'
		});

		expect(
			describeChronicleEntry(
				entry({
					entry_type: 'shipment_arrived',
					related_household_name: 'Hrafnstead',
					data: { resource_type: 'provisions', quantity_milli: 30_000 }
				})
			)
		).toEqual({
			title: 'Shipment arrived',
			detail: '30 provisions arrived from Hrafnstead.'
		});
	});

	it('renders assignment scheduling and completion facts', () => {
		expect(
			describeChronicleEntry(
				entry({
					entry_type: 'assignment_scheduled',
					subject_character_name: 'Astrid',
					data: { activity: 'fishing', starts_tick: 5, ends_tick: 7 }
				})
			)
		).toEqual({
			title: 'Work scheduled',
			detail: 'Astrid was assigned to fishing for ticks 5–7.'
		});

		expect(
			describeChronicleEntry(
				entry({
					entry_type: 'assignment_completed',
					subject_character_name: 'Astrid',
					data: { activity: 'fishing' }
				})
			)
		).toEqual({ title: 'Work completed', detail: 'Astrid completed fishing.' });
	});
});
