import { describe, expect, it } from 'vitest';
import type { CalendarEvent } from '$lib/api/generated';
import { calendarEventLabel } from './calendar';

const event = (overrides: Partial<CalendarEvent>): CalendarEvent => ({
	id: 'calendar-event',
	game_day: 0,
	category: 'world',
	kind: 'festival',
	importance: 'context',
	action_required: false,
	...overrides
});

describe('calendar event presentation', () => {
	it('renders stable historical anchor codes in the frontend', () => {
		expect(calendarEventLabel(event({ code: 'jol' }))).toBe('Jól');
		expect(calendarEventLabel(event({ code: 'thing', kind: 'assembly' }))).toBe('Þing');
		expect(calendarEventLabel(event({ code: 'summer', kind: 'season_start' }))).toBe(
			'Summer begins'
		);
		expect(calendarEventLabel(event({ code: 'harvest_start', kind: 'harvest' }))).toBe(
			'Harvest begins'
		);
	});

	it('renders sourced event kinds without backend prose', () => {
		expect(calendarEventLabel(event({ kind: 'delivery_due', category: 'contract' }))).toBe(
			'Delivery due'
		);
		expect(calendarEventLabel(event({ kind: 'dispatch_deadline', category: 'contract' }))).toBe(
			'Dispatch shipment'
		);
		expect(
			calendarEventLabel(
				event({ kind: 'shipment_arrival', category: 'shipment', status: 'arrived' })
			)
		).toBe('Shipment arrived');
		expect(
			calendarEventLabel(
				event({ kind: 'shipment_arrival', category: 'shipment', status: 'in_transit' })
			)
		).toBe('Shipment expected');
		expect(calendarEventLabel(event({ kind: 'political_deadline', category: 'politics' }))).toBe(
			'Answer the Jarl'
		);
		expect(calendarEventLabel(event({ kind: 'assignment_end', category: 'farm' }))).toBe(
			'Work plan ends'
		);
	});
});
