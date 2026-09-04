import type { PageServerLoad } from './$types';

// The household layout owns the shared report load. The landing page has no
// write actions: detailed commands live with their owning screen.
export const load: PageServerLoad = async ({ parent }) => {
	const { report } = await parent();
	return { report };
};
