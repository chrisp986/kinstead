import { changeOccupation, occupationPreview } from '$lib/server/household-actions';
import { getHouseholdWorkPlan } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import type { Actions } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const result = await getHouseholdWorkPlan({
		client: createServerApi(fetch),
		path: { householdId: params.householdId }
	});
	if (!result.data)
		error(
			result.response?.status ?? 502,
			apiErrorMessage(result.error, 'Unable to load the work plan.')
		);
	return { workPlan: result.data };
};

export const actions = { changeOccupation, occupationPreview } satisfies Actions;
