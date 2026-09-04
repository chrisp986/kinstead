import {
	createHouseholdAssignment,
	dispatchContractObligation,
	purchaseMarketOffer,
	proposeContract as proposeContractApi,
	respondToContract,
	respondToPoliticalDemand,
	type CreateAssignmentIntent
} from '$lib/api/generated';
import { parseMilli } from '$lib/domain/format';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { fail } from '@sveltejs/kit';

type ActionContext = { fetch: typeof fetch; params: { householdId: string }; request: Request };

export async function assign({ fetch, params, request }: ActionContext) {
	const formData = await request.formData();
	const duration = Number(formData.get('duration_ticks'));
	const intent: CreateAssignmentIntent = {
		character_id: String(formData.get('character_id') ?? ''),
		activity: String(formData.get('activity') ?? '') as CreateAssignmentIntent['activity'],
		intensity: String(formData.get('intensity') ?? '') as CreateAssignmentIntent['intensity'],
		duration_ticks: duration as CreateAssignmentIntent['duration_ticks']
	};
	if (
		!intent.character_id ||
		!['agriculture', 'fishing', 'woodcutting', 'rest'].includes(intent.activity) ||
		!['light', 'normal', 'high'].includes(intent.intensity) ||
		![1, 3, 6, 12].includes(duration)
	)
		return fail(400, {
			action: 'assign',
			message: 'Choose a valid member, activity, intensity, and duration.'
		});
	try {
		const result = await createHouseholdAssignment({
			client: createServerApi(fetch),
			path: { householdId: params.householdId },
			body: intent
		});
		if (!result.data)
			return fail(result.response?.status ?? 502, {
				action: 'assign',
				message: apiErrorMessage(result.error, 'The work plan could not be scheduled.')
			});
		return {
			success: true,
			action: 'assign',
			message: `${result.data.character}'s work was scheduled.`
		};
	} catch {
		return fail(503, { action: 'assign', message: 'The simulation backend is unavailable.' });
	}
}

export async function purchase({ fetch, params, request }: ActionContext) {
	const formData = await request.formData();
	const offerId = String(formData.get('offer_id') ?? '');
	const quantityMilli = parseMilli(String(formData.get('quantity') ?? ''));
	if (!offerId || quantityMilli === null)
		return fail(400, {
			action: 'purchase',
			message: 'Enter a positive quantity with at most three decimal places.'
		});
	try {
		const result = await purchaseMarketOffer({
			client: createServerApi(fetch),
			path: { offerId },
			body: { buyer_household_id: params.householdId, quantity_milli: quantityMilli }
		});
		if (!result.data)
			return fail(result.response?.status ?? 502, {
				action: 'purchase',
				message: apiErrorMessage(result.error, 'The purchase could not be completed.')
			});
		return {
			success: true,
			action: 'purchase',
			message: 'Purchase accepted. The shipment is on its way.'
		};
	} catch {
		return fail(503, { action: 'purchase', message: 'The simulation backend is unavailable.' });
	}
}

export async function proposeContract({ fetch, params, request }: ActionContext) {
	const formData = await request.formData();
	const counterpartyHouseholdId = String(formData.get('counterparty_household_id') ?? '');
	const resourceType = String(formData.get('resource_type') ?? '');
	const quantityMilli = parseMilli(String(formData.get('quantity') ?? ''));
	const currentGameDay = Number(formData.get('current_game_day'));
	const firstDueOffset = Number(formData.get('first_due_offset'));
	const intervalDays = Number(formData.get('interval_days'));
	const endConditionType = String(formData.get('end_condition_type') ?? '');
	const deliveryCount = Number(formData.get('delivery_count'));
	const firstDueGameDay = currentGameDay + firstDueOffset;
	if (
		!counterpartyHouseholdId ||
		counterpartyHouseholdId === params.householdId ||
		!['provisions', 'wood', 'trade_goods', 'silver'].includes(resourceType) ||
		quantityMilli === null ||
		!Number.isSafeInteger(currentGameDay) ||
		!Number.isSafeInteger(firstDueOffset) ||
		!Number.isSafeInteger(intervalDays) ||
		!Number.isSafeInteger(firstDueGameDay) ||
		firstDueOffset < 1 ||
		![7, 14, 28].includes(intervalDays) ||
		!['fixed_delivery_count', 'winter_start', 'summer_start'].includes(endConditionType) ||
		(endConditionType === 'fixed_delivery_count' &&
			(!Number.isSafeInteger(deliveryCount) || deliveryCount < 1))
	)
		return fail(400, {
			action: 'proposeContract',
			message: 'Check the contract terms and calendar schedule.'
		});
	const endCondition =
		endConditionType === 'fixed_delivery_count'
			? { type: 'fixed_delivery_count' as const, delivery_count: deliveryCount }
			: { type: endConditionType as 'winter_start' | 'summer_start' };
	try {
		const result = await proposeContractApi({
			client: createServerApi(fetch),
			body: {
				proposer_household_id: params.householdId,
				counterparty_household_id: counterpartyHouseholdId,
				first_due_game_day: firstDueGameDay,
				interval_days: intervalDays as 7 | 14 | 28,
				end_condition: endCondition,
				terms: [
					{
						debtor_household_id: params.householdId,
						creditor_household_id: counterpartyHouseholdId,
						resource_type: resourceType,
						quantity_milli: quantityMilli
					}
				]
			}
		});
		if (!result.data)
			return fail(result.response?.status ?? 502, {
				action: 'proposeContract',
				message: apiErrorMessage(result.error, 'The contract could not be proposed.')
			});
		return { success: true, action: 'proposeContract', message: 'Contract proposal sent.' };
	} catch {
		return fail(503, {
			action: 'proposeContract',
			message: 'The simulation backend is unavailable.'
		});
	}
}

export async function respondContract({ fetch, params, request }: ActionContext) {
	const formData = await request.formData();
	const contractId = String(formData.get('contract_id') ?? '');
	const decision = String(formData.get('decision') ?? '');
	if (!contractId || !['accept', 'reject'].includes(decision))
		return fail(400, { action: 'respondContract', message: 'Choose accept or reject.' });
	try {
		const result = await respondToContract({
			client: createServerApi(fetch),
			path: { contractId },
			body: {
				counterparty_household_id: params.householdId,
				decision: decision as 'accept' | 'reject'
			}
		});
		if (!result.data)
			return fail(result.response?.status ?? 502, {
				action: 'respondContract',
				message: apiErrorMessage(result.error, 'The contract response could not be recorded.')
			});
		return {
			success: true,
			action: 'respondContract',
			message: decision === 'accept' ? 'Contract accepted.' : 'Contract rejected.'
		};
	} catch {
		return fail(503, {
			action: 'respondContract',
			message: 'The simulation backend is unavailable.'
		});
	}
}

export async function dispatchObligation({ fetch, params, request }: ActionContext) {
	const obligationId = String((await request.formData()).get('obligation_id') ?? '');
	if (!obligationId)
		return fail(400, {
			action: 'dispatchObligation',
			message: 'Choose an obligation to dispatch.'
		});
	try {
		const result = await dispatchContractObligation({
			client: createServerApi(fetch),
			path: { obligationId },
			body: { debtor_household_id: params.householdId }
		});
		if (!result.data)
			return fail(result.response?.status ?? 502, {
				action: 'dispatchObligation',
				message: apiErrorMessage(result.error, 'The promised goods could not be dispatched.')
			});
		return {
			success: true,
			action: 'dispatchObligation',
			message: 'Shipment dispatched. It is on the way.'
		};
	} catch {
		return fail(503, {
			action: 'dispatchObligation',
			message: 'The simulation backend is unavailable.'
		});
	}
}

export async function respondPoliticalDemand({ fetch, params, request }: ActionContext) {
	const formData = await request.formData();
	const decisionId = String(formData.get('decision_id') ?? '');
	const option = String(formData.get('option') ?? '');
	const characterId = String(formData.get('character_id') ?? '');
	if (!decisionId || !['serve', 'pay_wood', 'pay_silver', 'refuse'].includes(option))
		return fail(400, {
			action: 'respondPoliticalDemand',
			message: 'Choose a valid demand option.'
		});
	try {
		const result = await respondToPoliticalDemand({
			client: createServerApi(fetch),
			path: { decisionId },
			body: {
				household_id: params.householdId,
				option: option as 'serve' | 'pay_wood' | 'pay_silver' | 'refuse',
				...(characterId ? { character_id: characterId } : {})
			}
		});
		if (!result.data)
			return fail(result.response?.status ?? 502, {
				action: 'respondPoliticalDemand',
				message: apiErrorMessage(result.error, 'The demand response could not be recorded.')
			});
		return { success: true, action: 'respondPoliticalDemand', message: 'Jarl demand resolved.' };
	} catch {
		return fail(503, {
			action: 'respondPoliticalDemand',
			message: 'The simulation backend is unavailable.'
		});
	}
}
