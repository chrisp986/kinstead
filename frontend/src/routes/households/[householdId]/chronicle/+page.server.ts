import { listHouseholdChronicle } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const result = await listHouseholdChronicle({
		client: createServerApi(fetch),
		path: { householdId: params.householdId }
	});
	if (!result.data)
		error(
			result.response?.status ?? 502,
			apiErrorMessage(result.error, 'Unable to load household chronicle')
		);
	return { chronicle: result.data.entries };
};
