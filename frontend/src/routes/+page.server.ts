import { error, redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
	const response = await fetch(`${env.BACKEND_URL ?? 'http://localhost:8080'}/api/session`);
	if (response.status === 401) redirect(303, '/sign-in');
	if (!response.ok) error(response.status, 'Unable to load your household.');
	const session = (await response.json()) as { households: { id: string; name: string }[] };
	if (!session.households.length) error(403, 'No household has been assigned to this player.');
	redirect(307, `/households/${session.households[0].id}`);
};
