import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch, persona: 'leader-yggdrasil' });

	const search = url.searchParams.get('search') || undefined;
	const category_id = url.searchParams.get('category') || undefined;
	const location_id = url.searchParams.get('location') || undefined;

	const [articles, locations, categories] = await Promise.all([
		api.listArticles({ search, category_id, location_id }),
		api.listLocations(),
		api.listCategories()
	]);

	return { articles, locations, categories, filters: { search, category_id, location_id } };
};
