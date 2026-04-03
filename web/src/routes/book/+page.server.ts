import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
	const api = createApiClient({ fetch, persona: 'leader-yggdrasil' });
	const [locations, categories, units] = await Promise.all([
		api.listLocations(),
		api.listCategories(),
		api.listUnits()
	]);
	return { locations, categories, units };
};
