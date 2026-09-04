import { getHouseholdCalendar } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const result = await getHouseholdCalendar({
		client: createServerApi(fetch),
		path: { householdId: params.householdId }
	});
	if (!result.data)
		error(result.response?.status ?? 502, apiErrorMessage(result.error, 'Unable to load calendar'));
	return { calendar: result.data };
};
