import { listAdminHouseholds, listAdminWorlds } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const client = createServerApi(fetch);
	const [worlds, households] = await Promise.all([
		listAdminWorlds({ client, query: { limit: 50 } }),
		listAdminHouseholds({ client, query: { q: url.searchParams.get('q') ?? '', limit: 50 } })
	]);
	if (!worlds.data || !households.data)
		error(
			503,
			apiErrorMessage(worlds.error ?? households.error, 'Unable to load console overview.')
		);
	return {
		worlds: worlds.data.worlds,
		households: households.data.households,
		query: url.searchParams.get('q') ?? ''
	};
};
