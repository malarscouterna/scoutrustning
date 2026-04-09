import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const { user } = await parent();
	if (!user) return { locations: [], categories: [], groupSettings: null };

	const api = createApiClient({ fetch });
	const isManager = user.roles.includes('equipment_manager');

	const [locations, categories] = await Promise.all([
		api.listLocations(),
		api.listCategories()
	]);

	let groupSettings = null;
	if (isManager) {
		try {
			groupSettings = await api.getGroupSettings();
		} catch { /* defaults */ }
	}

	return { locations, categories, groupSettings };
};
