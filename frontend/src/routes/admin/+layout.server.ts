import { getSession } from '$lib/api/generated';
import { createServerApi } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ fetch, setHeaders }) => {
	setHeaders({ 'cache-control': 'no-store' });
	const result = await getSession({ client: createServerApi(fetch) });
	if (result.response?.status === 401) redirect(303, '/sign-in');
	if (!result.data)
		error(result.response?.status ?? 503, 'Unable to verify developer-console access.');
	if (!result.data.is_admin) error(403, 'Developer-console access is required.');
	return { session: result.data };
};
