import { getAdminHousehold, listAdminHouseholdTicks } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const client = createServerApi(fetch);
	const [household, ticks] = await Promise.all([
		getAdminHousehold({ client, path: { id: params.householdId } }),
		listAdminHouseholdTicks({ client, path: { id: params.householdId }, query: { limit: 20 } })
	]);
	if (household.response?.status === 401 || ticks.response?.status === 401)
		redirect(303, '/sign-in');
	if (!household.data || !ticks.data)
		error(503, apiErrorMessage(household.error ?? ticks.error, 'Unable to load household.'));
	return {
		household: household.data,
		diagnostics: ticks.data.diagnostics,
		retainedDays: ticks.data.retained_days
	};
};
