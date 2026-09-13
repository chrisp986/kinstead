import { getAdminAccount, listAdminSessions } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const client = createServerApi(fetch);
	const [account, sessions] = await Promise.all([
		getAdminAccount({ client, path: { id: params.playerId } }),
		listAdminSessions({ client, path: { id: params.playerId }, query: { limit: 50 } })
	]);
	if (account.response?.status === 401 || sessions.response?.status === 401)
		redirect(303, '/sign-in');
	if (!account.data || !sessions.data)
		error(503, apiErrorMessage(account.error ?? sessions.error, 'Unable to load account.'));
	return { account: account.data, sessions: sessions.data.sessions };
};
