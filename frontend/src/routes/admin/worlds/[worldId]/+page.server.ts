import { getAdminWorld } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const result = await getAdminWorld({
		client: createServerApi(fetch),
		path: { id: params.worldId }
	});
	if (result.response?.status === 401) redirect(303, '/sign-in');
	if (!result.data)
		error(result.response?.status ?? 503, apiErrorMessage(result.error, 'Unable to load world.'));
	return { world: result.data };
};
