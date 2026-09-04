import { getHouseholdReport } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ fetch, params }) => {
	try {
		const result = await getHouseholdReport({
			client: createServerApi(fetch),
			path: { householdId: params.householdId }
		});
		if (!result.data) {
			error(
				result.response?.status ?? 502,
				apiErrorMessage(result.error, 'Unable to load household')
			);
		}
		return { report: result.data };
	} catch (cause) {
		if (cause && typeof cause === 'object' && 'status' in cause) throw cause;
		error(503, 'The simulation backend is unavailable. Start the Go API and try again.');
	}
};
