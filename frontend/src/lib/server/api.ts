import { env } from '$env/dynamic/private';
import { createClient } from '$lib/api/generated/client';
import type { ApiError } from '$lib/api/generated';

const defaultBackendUrl = 'http://localhost:8080';

export function createServerApi(fetchImplementation: typeof fetch) {
	return createClient({
		baseUrl: env.BACKEND_URL ?? defaultBackendUrl,
		fetch: fetchImplementation,
		responseStyle: 'fields'
	});
}

export function apiErrorMessage(value: unknown, fallback: string): string {
	if (isApiError(value)) return value.message ?? humanize(value.error);
	return fallback;
}

function isApiError(value: unknown): value is ApiError {
	return typeof value === 'object' && value !== null && 'error' in value;
}

function humanize(value: unknown): string {
	return String(value).replaceAll('_', ' ');
}
