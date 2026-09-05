import { formatMilli, labelResource } from './format';
import { formatRelativeGameDay } from './time';

type ReportItem = {
	code: string;
	severity: string;
	target?: string;
	related_id?: string;
	due_game_day?: number;
	data?: Record<string, unknown>;
};

export function describeReportItem(item: ReportItem, currentGameDay?: number): string {
	const data = item.data ?? {};
	const days = typeof data.supply_days === 'number' ? data.supply_days.toFixed(1) : undefined;
	const name = typeof data.character_name === 'string' ? data.character_name : 'A household member';
	const actor = typeof data.actor_name === 'string' ? data.actor_name : 'the Jarl';
	const due =
		item.due_game_day === undefined || currentGameDay === undefined
			? 'the deadline'
			: formatRelativeGameDay(currentGameDay, item.due_game_day);
	const resource =
		typeof data.resource_type === 'string'
			? labelResource(data.resource_type).toLowerCase()
			: 'goods';
	const quantity =
		typeof data.quantity_milli === 'number' ? `${formatMilli(data.quantity_milli)} ` : '';
	switch (item.code) {
		case 'supply_emergency':
			return `Provisions will last less than 7 days${days ? ` (about ${days})` : ''}.`;
		case 'supply_critical':
			return `Provisions will last about ${days ?? 'few'} days.`;
		case 'supply_strained':
			return `Provisions are strained at about ${days ?? '30'} days.`;
		case 'character_fatigue_critical':
			return `${name} is dangerously fatigued.`;
		case 'character_fatigue_high':
			return `${name} is heavily fatigued.`;
		case 'political_demand_due':
			return `Respond to ${actor} before ${due}.`;
		case 'contract_obligation_due':
			return `Dispatch ${quantity}${resource} before ${due}.`;
		case 'respond_political_demand':
			return `Respond to ${actor} before ${due}.`;
		case 'dispatch_contract_obligation':
			return `Dispatch ${quantity}${resource} before ${due}.`;
		case 'secure_provisions':
			return 'Secure additional provisions.';
		case 'rest_fatigued_character':
			return `Plan rest for ${name}.`;
		default:
			return 'Review the household situation.';
	}
}
