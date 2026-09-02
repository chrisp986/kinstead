import {
	createHouseholdAssignment,
	getHouseholdReport,
	listHouseholdChronicle,
	listHouseholdShipments,
	listMarketOffers,
	purchaseMarketOffer,
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

		const [shipmentResult, marketResult, chronicleResult] = await Promise.all([
			listHouseholdShipments({ client, path: { householdId: params.householdId } }),
			listMarketOffers({ client, query: { world_id: reportResult.data.world_id } }),
			listHouseholdChronicle({ client, path: { householdId: params.householdId } })
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

		return {
			report: reportResult.data,
			shipments: shipmentResult.data.shipments,
			offers: marketResult.data.offers,
			chronicle: chronicleResult.data.entries
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
	}
} satisfies Actions;
