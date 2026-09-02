import { redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

const developmentHouseholdId = '00000000-0000-0000-0000-000000000020';

export function load() {
	redirect(307, `/households/${env.DEFAULT_HOUSEHOLD_ID ?? developmentHouseholdId}`);
}
