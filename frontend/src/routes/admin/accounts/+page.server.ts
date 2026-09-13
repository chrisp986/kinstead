import { listAdminAccounts } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const result = await listAdminAccounts({
		client: createServerApi(fetch),
		query: { q: url.searchParams.get('q') ?? '', limit: 50 }
	});
	if (result.response?.status === 401) redirect(303, '/sign-in');
	if (!result.data)
		error(
			result.response?.status ?? 503,
			apiErrorMessage(result.error, 'Unable to load accounts.')
		);
	return { accounts: result.data.accounts, query: url.searchParams.get('q') ?? '' };
};
