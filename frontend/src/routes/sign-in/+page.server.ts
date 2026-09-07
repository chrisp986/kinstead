import { dev } from '$app/environment';
import { env } from '$env/dynamic/private';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';

export const actions: Actions = {
	default: async ({ request, fetch, cookies }) => {
		const data = await request.formData();
		const token = String(data.get('session') ?? '');
		if (!/^[A-Za-z0-9_-]{43}$/.test(token))
			return fail(400, { message: 'Enter a valid session key.' });
		const response = await fetch(`${env.BACKEND_URL ?? 'http://localhost:8080'}/api/session`, {
			headers: { authorization: `Bearer ${token}` }
		});
		if (!response.ok) return fail(401, { message: 'This session has expired or is unavailable.' });
		cookies.set('game_session', token, {
			path: '/',
			httpOnly: true,
			secure: !dev,
			sameSite: 'lax'
		});
		redirect(303, '/');
	}
};
