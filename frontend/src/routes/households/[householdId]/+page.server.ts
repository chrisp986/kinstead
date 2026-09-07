import { acknowledgeHouseholdReport, getHouseholdCalendar } from '$lib/api/generated';
import { apiErrorMessage, createServerApi } from '$lib/server/api';
import { error } from '@sveltejs/kit';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

// The household layout owns the shared report load. The landing page has no
// write actions: detailed commands live with their owning screen.
export const load: PageServerLoad = async ({ fetch, params, parent }) => {
	const { report } = await parent();
	const result = await getHouseholdCalendar({
		client: createServerApi(fetch),
		path: { householdId: params.householdId }
	});
	if (!result.data)
		error(result.response?.status ?? 502, apiErrorMessage(result.error, 'Unable to load calendar'));
	return { report, calendar: result.data };
};

export const actions = {
	acknowledge: async ({ fetch, params, request }) => {
		const form = await request.formData();
		const gameDay = Number(form.get('game_day'));
		const cursorValue = form.get('chronicle_cursor');
		const chronicleCursor = cursorValue === null ? undefined : Number(cursorValue);
		const result = await acknowledgeHouseholdReport({
			client: createServerApi(fetch),
			path: { householdId: params.householdId },
			body: { game_day: gameDay, chronicle_cursor: chronicleCursor }
		});
		if (result.response?.status !== 204)
			return fail(result.response?.status ?? 502, {
				message: apiErrorMessage(result.error, 'Could not acknowledge this report.')
			});
		return { success: true };
	}
} satisfies Actions;
