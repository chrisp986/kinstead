import { getHouseholdPolitics } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import { respondPoliticalDemand } from '$lib/server/household-actions';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const result = await getHouseholdPolitics({
		client: createServerApi(fetch),
		path: { householdId: params.householdId }
	});
	if (!result.data)
		error(result.response?.status ?? 502, apiErrorMessage(result.error, 'Unable to load politics'));
	return { politics: result.data };
};

export const actions = { respondPoliticalDemand } satisfies Actions;
