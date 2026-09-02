import type { ChronicleEntry } from '$lib/api/generated';
import { formatMilli, labelActivity, labelResource, sentenceCase } from './format';

export type ChronicleDescription = {
	title: string;
	detail: string;
};

export function describeChronicleEntry(entry: ChronicleEntry): ChronicleDescription {
	const resource = dataString(entry, 'resource_type');
	const quantity = dataInteger(entry, 'quantity_milli');
	const cost = dataInteger(entry, 'cost_milli');
	const activity = dataString(entry, 'activity');
	const startsTick = dataInteger(entry, 'starts_tick');
	const endsTick = dataInteger(entry, 'ends_tick');
	const person = entry.subject_character_name ?? 'A household member';
	const otherHousehold = entry.related_household_name ?? 'another household';

	switch (entry.entry_type) {
		case 'assignment_scheduled':
			return {
				title: 'Work scheduled',
				detail: `${person} was assigned to ${activityLabel(activity)}${tickRange(startsTick, endsTick)}.`
			};
		case 'assignment_completed':
			return {
				title: 'Work completed',
				detail: `${person} completed ${activityLabel(activity)}.`
			};
		case 'market_purchase':
			return {
				title: 'Market purchase',
				detail: `Bought ${resourceAmount(quantity, resource)} from ${otherHousehold}${costPhrase(cost)}.`
			};
		case 'market_sale':
			return {
				title: 'Market sale',
				detail: `Sold ${resourceAmount(quantity, resource)} to ${otherHousehold}${costPhrase(cost)}.`
			};
		case 'shipment_arrived':
			return {
				title: 'Shipment arrived',
				detail: `${resourceAmount(quantity, resource)} arrived from ${otherHousehold}.`
			};
		default:
			return {
				title: sentenceCase(entry.entry_type),
				detail: 'A household event was recorded.'
			};
	}
}

function dataString(entry: ChronicleEntry, key: string): string | undefined {
	const value = entry.data[key];
	return typeof value === 'string' ? value : undefined;
}

function dataInteger(entry: ChronicleEntry, key: string): number | undefined {
	const value = entry.data[key];
	return typeof value === 'number' && Number.isSafeInteger(value) ? value : undefined;
}

function activityLabel(activity: string | undefined): string {
	return activity ? labelActivity(activity).toLowerCase() : 'work';
}

function resourceAmount(quantity: number | undefined, resource: string | undefined): string {
	const amount = quantity === undefined ? 'Goods' : formatMilli(quantity);
	const name = resource ? labelResource(resource).toLowerCase() : 'goods';
	return `${amount} ${name}`;
}

function costPhrase(cost: number | undefined): string {
	return cost === undefined ? '' : ` for ${formatMilli(cost)} silver`;
}

function tickRange(startsTick: number | undefined, endsTick: number | undefined): string {
	if (startsTick === undefined || endsTick === undefined) return '';
	return startsTick === endsTick
		? ` on tick ${startsTick}`
		: ` for ticks ${startsTick}–${endsTick}`;
}
