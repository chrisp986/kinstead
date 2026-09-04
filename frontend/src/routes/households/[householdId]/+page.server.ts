import { getHouseholdCalendar } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

// The household layout owns the shared report load. The landing page has no
// write actions: detailed commands live with their owning screen.
export const load: PageServerLoad = async ({ fetch, params, parent }) => {
	const { report } = await parent();
	const result = await getHouseholdCalendar({
		client: createServerApi(fetch),
		path: { householdId: params.householdId }
	});
	if (!result.data)
		error(result.response?.status ?? 502, apiErrorMessage(result.error, 'Unable to load calendar'));
	return { report, calendar: result.data };
};
