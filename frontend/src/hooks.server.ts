import { env } from '$env/dynamic/private';
import type { HandleFetch } from '@sveltejs/kit';

export const handleFetch: HandleFetch = async ({ event, request, fetch }) => {
	const backend = new URL(env.BACKEND_URL ?? 'http://localhost:8080');
	const target = new URL(request.url);
	if (target.origin === backend.origin && target.pathname.startsWith('/api/')) {
		const token = event.cookies.get('game_session');
		if (token && !request.headers.has('authorization')) {
			request.headers.set('authorization', `Bearer ${token}`);
		}
	}
	return fetch(request);
};
