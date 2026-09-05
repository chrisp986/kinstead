import type { CalendarEvent } from '$lib/api/generated';
import { formatPhaseName } from './time';

const anchorLabels: Record<string, string> = {
	jol: 'Jól',
	thing: 'Þing',
	midsummer: 'Midsummer',
	harvest_start: 'Harvest begins',
	summer_start: 'Summer begins',
	winter_start: 'Winter begins',
	midwinter: 'Midwinter'
};

export function calendarEventLabel(event: CalendarEvent): string {
	switch (event.kind) {
		case 'season_start':
			return `${formatPhaseName(event.code ?? '')} begins`;
		case 'festival':
			return anchorLabels[event.code ?? ''] ?? 'Festival';
		case 'harvest':
			return anchorLabels[event.code ?? ''] ?? 'Harvest';
		case 'delivery_due':
			return 'Delivery due';
		case 'dispatch_deadline':
			return 'Dispatch shipment';
		case 'shipment_arrival':
			switch (event.status) {
				case 'arrived':
					return 'Shipment arrived';
				case 'in_transit':
					return 'Shipment expected';
				default:
					return 'Shipment';
			}
		case 'political_deadline':
			return 'Answer the Jarl';
		case 'assignment_end':
			return 'Work plan ends';
		case 'assembly':
			return anchorLabels[event.code ?? ''] ?? 'Assembly';
		default:
			return formatPhaseName(event.kind);
	}
}
