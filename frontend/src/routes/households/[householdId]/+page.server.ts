import {
	createHouseholdAssignment,
	dispatchContractObligation,
	getHouseholdReport,
	listHouseholdChronicle,
	listHouseholdContracts,
	listHouseholdRelationships,
	listHouseholdShipments,
	listMarketOffers,
	proposeContract,
	purchaseMarketOffer,
	respondToContract,
	type CreateAssignmentIntent
} from '$lib/api/generated';
import { parseMilli } from '$lib/domain/format';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const client = createServerApi(fetch);
	try {
		const reportResult = await getHouseholdReport({
			client,
			path: { householdId: params.householdId }
		});
		if (!reportResult.data) {
			error(
				reportResult.response?.status ?? 502,
				apiErrorMessage(reportResult.error, 'Unable to load household')
			);
		}

		const [shipmentResult, marketResult, chronicleResult, contractResult, relationshipResult] =
			await Promise.all([
				listHouseholdShipments({ client, path: { householdId: params.householdId } }),
				listMarketOffers({ client, query: { world_id: reportResult.data.world_id } }),
				listHouseholdChronicle({ client, path: { householdId: params.householdId } }),
				listHouseholdContracts({ client, path: { householdId: params.householdId } }),
				listHouseholdRelationships({ client, path: { householdId: params.householdId } })
			]);
		if (!shipmentResult.data)
			error(shipmentResult.response?.status ?? 502, 'Unable to load shipments');
		if (!marketResult.data) {
			error(
				marketResult.response?.status ?? 502,
				apiErrorMessage(marketResult.error, 'Unable to load market offers')
			);
		}
		if (!chronicleResult.data) {
			error(
				chronicleResult.response?.status ?? 502,
				apiErrorMessage(chronicleResult.error, 'Unable to load household chronicle')
			);
		}
		if (!contractResult.data) {
			error(
				contractResult.response?.status ?? 502,
				apiErrorMessage(contractResult.error, 'Unable to load household contracts')
			);
		}
		if (!relationshipResult.data) {
			error(
				relationshipResult.response?.status ?? 502,
				apiErrorMessage(relationshipResult.error, 'Unable to load household relationships')
			);
		}

		return {
			report: reportResult.data,
			shipments: shipmentResult.data.shipments,
			offers: marketResult.data.offers,
			chronicle: chronicleResult.data.entries,
			contracts: contractResult.data.contracts,
			relationships: relationshipResult.data.relationships
		};
	} catch (cause) {
		if (cause && typeof cause === 'object' && 'status' in cause) throw cause;
		error(503, 'The simulation backend is unavailable. Start the Go API and try again.');
	}
};

export const actions = {
	assign: async ({ fetch, params, request }) => {
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
		) {
			return fail(400, {
				action: 'assign',
				message: 'Choose a valid member, activity, intensity, and duration.'
			});
		}

		try {
			const result = await createHouseholdAssignment({
				client: createServerApi(fetch),
				path: { householdId: params.householdId },
				body: intent
			});
			if (!result.data) {
				return fail(result.response?.status ?? 502, {
					action: 'assign',
					message: apiErrorMessage(result.error, 'The work plan could not be scheduled.')
				});
			}
			return {
				success: true,
				action: 'assign',
				message: `${result.data.character}'s work was scheduled.`
			};
		} catch {
			return fail(503, { action: 'assign', message: 'The simulation backend is unavailable.' });
		}
	},

	purchase: async ({ fetch, params, request }) => {
		const formData = await request.formData();
		const offerId = String(formData.get('offer_id') ?? '');
		const quantityMilli = parseMilli(String(formData.get('quantity') ?? ''));
		if (!offerId || quantityMilli === null) {
			return fail(400, {
				action: 'purchase',
				message: 'Enter a positive quantity with at most three decimal places.'
			});
		}

		try {
			const result = await purchaseMarketOffer({
				client: createServerApi(fetch),
				path: { offerId },
				body: { buyer_household_id: params.householdId, quantity_milli: quantityMilli }
			});
			if (!result.data) {
				return fail(result.response?.status ?? 502, {
					action: 'purchase',
					message: apiErrorMessage(result.error, 'The purchase could not be completed.')
				});
			}
			return {
				success: true,
				action: 'purchase',
				message: `Purchase accepted. Shipment is expected at tick ${result.data.shipment.expected_arrival_tick}.`
			};
		} catch {
			return fail(503, { action: 'purchase', message: 'The simulation backend is unavailable.' });
		}
	},

	proposeContract: async ({ fetch, params, request }) => {
		const formData = await request.formData();
		const counterpartyHouseholdId = String(formData.get('counterparty_household_id') ?? '');
		const resourceType = String(formData.get('resource_type') ?? '');
		const quantityMilli = parseMilli(String(formData.get('quantity') ?? ''));
		const startsTick = Number(formData.get('starts_tick'));
		const endsTick = Number(formData.get('ends_tick'));
		const intervalTicks = Number(formData.get('interval_ticks'));
		if (
			!counterpartyHouseholdId ||
			counterpartyHouseholdId === params.householdId ||
			!['provisions', 'wood', 'trade_goods', 'silver'].includes(resourceType) ||
			quantityMilli === null ||
			!Number.isSafeInteger(startsTick) ||
			!Number.isSafeInteger(endsTick) ||
			!Number.isSafeInteger(intervalTicks) ||
			startsTick < 1 ||
			endsTick < startsTick ||
			intervalTicks < 1
		) {
			return fail(400, {
				action: 'proposeContract',
				message: 'Check the contract terms and due ticks.'
			});
		}
		try {
			const result = await proposeContract({
				client: createServerApi(fetch),
				body: {
					proposer_household_id: params.householdId,
					counterparty_household_id: counterpartyHouseholdId,
					starts_tick: startsTick,
					ends_tick: endsTick,
					interval_ticks: intervalTicks,
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
			if (!result.data) {
				return fail(result.response?.status ?? 502, {
					action: 'proposeContract',
					message: apiErrorMessage(result.error, 'The contract could not be proposed.')
				});
			}
			return { success: true, action: 'proposeContract', message: 'Contract proposal sent.' };
		} catch {
			return fail(503, {
				action: 'proposeContract',
				message: 'The simulation backend is unavailable.'
			});
		}
	},

	respondContract: async ({ fetch, params, request }) => {
		const formData = await request.formData();
		const contractId = String(formData.get('contract_id') ?? '');
		const decision = String(formData.get('decision') ?? '');
		if (!contractId || !['accept', 'reject'].includes(decision)) {
			return fail(400, { action: 'respondContract', message: 'Choose accept or reject.' });
		}
		try {
			const result = await respondToContract({
				client: createServerApi(fetch),
				path: { contractId },
				body: {
					counterparty_household_id: params.householdId,
					decision: decision as 'accept' | 'reject'
				}
			});
			if (!result.data) {
				return fail(result.response?.status ?? 502, {
					action: 'respondContract',
					message: apiErrorMessage(result.error, 'The contract response could not be recorded.')
				});
			}
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
	},

	dispatchObligation: async ({ fetch, params, request }) => {
		const formData = await request.formData();
		const obligationId = String(formData.get('obligation_id') ?? '');
		if (!obligationId) {
			return fail(400, {
				action: 'dispatchObligation',
				message: 'Choose an obligation to dispatch.'
			});
		}
		try {
			const result = await dispatchContractObligation({
				client: createServerApi(fetch),
				path: { obligationId },
				body: { debtor_household_id: params.householdId }
			});
			if (!result.data) {
				return fail(result.response?.status ?? 502, {
					action: 'dispatchObligation',
					message: apiErrorMessage(result.error, 'The promised goods could not be dispatched.')
				});
			}
			return {
				success: true,
				action: 'dispatchObligation',
				message: `Shipment dispatched for arrival at tick ${result.data.shipment.expected_arrival_tick}.`
			};
		} catch {
			return fail(503, {
				action: 'dispatchObligation',
				message: 'The simulation backend is unavailable.'
			});
		}
	}
} satisfies Actions;
