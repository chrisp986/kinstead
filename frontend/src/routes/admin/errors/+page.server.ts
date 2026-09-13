import { listAdminErrors } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const source = url.searchParams.get('source');
	const result = await listAdminErrors({
		client: createServerApi(fetch),
		query: {
			world_id: url.searchParams.get('world_id') ?? undefined,
			household_id: url.searchParams.get('household_id') ?? undefined,
			request_id: url.searchParams.get('request_id') ?? undefined,
			source: source === 'api' || source === 'worker' ? source : undefined,
			limit: 50
		}
	});
	if (result.response?.status === 401) redirect(303, '/sign-in');
	if (!result.data)
		error(result.response?.status ?? 503, apiErrorMessage(result.error, 'Unable to load errors.'));
	return {
		errors: result.data.errors,
		filters: {
			worldId: url.searchParams.get('world_id') ?? '',
			householdId: url.searchParams.get('household_id') ?? '',
			requestId: url.searchParams.get('request_id') ?? '',
			source: source ?? ''
		}
	};
};
