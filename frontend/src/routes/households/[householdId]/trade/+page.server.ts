import {
	listHouseholdContracts,
	listHouseholdRelationships,
	listHouseholdShipments,
	listMarketOffers
} from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import {
	dispatchObligation,
	proposeContract,
	purchase,
	respondContract
} from '$lib/server/household-actions';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params, parent }) => {
	const { report } = await parent();
	const client = createServerApi(fetch);
	const [marketResult, contractsResult, shipmentsResult, relationshipsResult] = await Promise.all([
		listMarketOffers({ client, query: { world_id: report.world_id } }),
		listHouseholdContracts({ client, path: { householdId: params.householdId } }),
		listHouseholdShipments({ client, path: { householdId: params.householdId } }),
		listHouseholdRelationships({ client, path: { householdId: params.householdId } })
	]);
	if (!marketResult.data)
		error(
			marketResult.response?.status ?? 502,
			apiErrorMessage(marketResult.error, 'Unable to load market offers')
		);
	if (!contractsResult.data)
		error(
			contractsResult.response?.status ?? 502,
			apiErrorMessage(contractsResult.error, 'Unable to load contracts')
		);
	if (!shipmentsResult.data)
		error(
			shipmentsResult.response?.status ?? 502,
			apiErrorMessage(shipmentsResult.error, 'Unable to load shipments')
		);
	if (!relationshipsResult.data)
		error(
			relationshipsResult.response?.status ?? 502,
			apiErrorMessage(relationshipsResult.error, 'Unable to load relationships')
		);
	return {
		offers: marketResult.data.offers,
		contracts: contractsResult.data.contracts,
		shipments: shipmentsResult.data.shipments,
		relationships: relationshipsResult.data.relationships
	};
};

export const actions = {
	purchase,
	proposeContract,
	respondContract,
	dispatchObligation
} satisfies Actions;
