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
	const trustDelta = dataInteger(entry, 'trust_delta');
	const actor = dataString(entry, 'actor_name') ?? 'the Jarl';
	const option = dataString(entry, 'selected_option') ?? dataString(entry, 'option');

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
		case 'shipment_cancelled':
			return {
				title: 'Shipment cancelled',
				detail: `${resourceAmount(quantity, resource)} shipment to ${otherHousehold} was cancelled.`
			};
		case 'contract_proposed':
			return {
				title: 'Contract proposed',
				detail: `A recurring promise was proposed with ${otherHousehold}.`
			};
		case 'contract_accepted':
			return {
				title: 'Contract accepted',
				detail: `A recurring promise with ${otherHousehold} was accepted.`
			};
		case 'contract_rejected':
			return {
				title: 'Contract rejected',
				detail: `A recurring promise with ${otherHousehold} was rejected.`
			};
		case 'contract_shipment_dispatched':
			return {
				title: 'Promise dispatched',
				detail: `${resourceAmount(quantity, resource)} was dispatched to ${otherHousehold}${tickRange(startsTick, dataInteger(entry, 'expected_arrival_tick'))}.`
			};
		case 'contract_obligation_fulfilled':
			return {
				title: 'Promise fulfilled',
				detail: `${resourceAmount(quantity, resource)} reached ${otherHousehold}${arrivalPhrase(entry)}${deltaPhrase(trustDelta)}.`
			};
		case 'contract_obligation_late':
			return {
				title: 'Promise delivered late',
				detail: `${resourceAmount(quantity, resource)} arrived ${latePhrase(entry)}${deltaPhrase(trustDelta)}.`
			};
		case 'contract_obligation_broken':
			return {
				title: 'Promise broken',
				detail: `${resourceAmount(quantity, resource)} was not delivered${deltaPhrase(trustDelta)}.`
			};
		case 'political_demand_received':
			return {
				title: 'Jarl demand received',
				detail: `${actor} requested ${dataString(entry, 'demand_type') === 'political_labor_service' ? 'labor service' : 'a levy'}. Respond before tick ${dataInteger(entry, 'deadline_tick') ?? 'the deadline'}.`
			};
		case 'political_demand_resolved':
			return {
				title: 'Jarl demand resolved',
				detail: `${actor} demand was answered${option ? ` with ${option.replaceAll('_', ' ')}` : ''}${deltaPhrase(dataInteger(entry, 'standing_delta'), 'standing')}.`
			};
		case 'political_demand_auto_resolved':
			return {
				title: 'Jarl demand expired',
				detail: `${actor} demand expired and was refused${deltaPhrase(dataInteger(entry, 'standing_delta'), 'standing')}.`
			};
		case 'emergency_food_work_scheduled':
			return {
				title: 'Emergency work scheduled',
				detail: `${person} was assigned to ${activityLabel(activity)} for supplies${tickRange(startsTick, endsTick)}.`
			};
		case 'emergency_work_overridden':
			return {
				title: 'Emergency work overridden',
				detail: `${person}'s emergency ${activityLabel(dataString(entry, 'emergency_activity'))} was replaced with ${activityLabel(dataString(entry, 'replacement_activity'))}.`
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

function deltaPhrase(delta: number | undefined, noun = 'trust'): string {
	if (delta === undefined) return '';
	return ` ${delta >= 0 ? 'increased' : 'decreased'} ${noun} by ${Math.abs(delta)}.`;
}

function arrivalPhrase(entry: ChronicleEntry): string {
	const actual = dataInteger(entry, 'actual_arrival_tick');
	return actual === undefined ? '' : ` on tick ${actual}`;
}

function latePhrase(entry: ChronicleEntry): string {
	const due = dataInteger(entry, 'due_arrival_tick');
	const actual = dataInteger(entry, 'actual_arrival_tick');
	if (due !== undefined && actual !== undefined) return `on tick ${actual}, due on tick ${due}`;
	return 'late';
}

function tickRange(startsTick: number | undefined, endsTick: number | undefined): string {
	if (startsTick === undefined || endsTick === undefined) return '';
	return startsTick === endsTick
		? ` on tick ${startsTick}`
		: ` for ticks ${startsTick}–${endsTick}`;
}
