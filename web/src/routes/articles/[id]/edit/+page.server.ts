import { createApiClient } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent, params }) => {
	const { user } = await parent();
	if (!user?.roles.includes('equipment_manager')) throw redirect(302, '/browse');

	const api = createApiClient({ fetch });
	const [article, locations, categories] = await Promise.all([
		api.getArticle(params.id),
		api.listLocations(),
		api.listCategories()
	]);

	return { article, locations, categories };
};
