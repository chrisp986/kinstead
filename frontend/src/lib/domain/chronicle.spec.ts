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

	it('renders known production facts with structured consequences', () => {
		const known = [
			['shipment_cancelled', 'Shipment cancelled'],
			['contract_proposed', 'Contract proposed'],
			['contract_accepted', 'Contract accepted'],
			['contract_rejected', 'Contract rejected'],
			['contract_shipment_dispatched', 'Promise dispatched'],
			['contract_obligation_fulfilled', 'Promise fulfilled'],
			['contract_obligation_late', 'Promise delivered late'],
			['contract_obligation_broken', 'Promise broken'],
			['political_demand_received', 'Jarl demand received'],
			['political_demand_resolved', 'Jarl demand resolved'],
			['political_demand_auto_resolved', 'Jarl demand expired'],
			['emergency_food_work_scheduled', 'Emergency work scheduled'],
			['emergency_work_overridden', 'Emergency work overridden']
		] as const;
		for (const [entryType, title] of known) {
			const result = describeChronicleEntry(
				entry({
					entry_type: entryType,
					subject_character_name: 'Astrid',
					related_household_name: 'Hrafnstead',
					data: {
						resource_type: 'wood',
						quantity_milli: 10_000,
						trust_delta: -1,
						standing_delta: 10,
						actual_arrival_tick: 11,
						due_arrival_tick: 10,
						demand_type: 'political_labor_service',
						deadline_tick: 18,
						activity: 'fishing',
						starts_tick: 5,
						ends_tick: 7,
						emergency_activity: 'fishing',
						replacement_activity: 'woodcutting'
					}
				})
			);
			expect(result.title).toBe(title);
			expect(result.detail.length).toBeGreaterThan(0);
		}
	});

	it('keeps unknown facts safe', () => {
		expect(describeChronicleEntry(entry({ entry_type: 'future_event' })).title).toBe(
			'Future event'
		);
	});
});
