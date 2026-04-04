import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

const DEFAULT_STATUSES = 'ok,reported_usable,reported_unusable,under_repair,drying,new';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch });

	const search = url.searchParams.get('search') || undefined;
	const category_id = url.searchParams.get('category') || undefined;
	const location_id = url.searchParams.get('location') || undefined;
	const status = url.searchParams.get('status') || DEFAULT_STATUSES;

	const [articles, locations, categories] = await Promise.all([
		api.listArticles({ search, category_id, location_id, status }),
		api.listLocations(),
		api.listCategories()
	]);

	return { articles, locations, categories, filters: { search, category_id, location_id, status } };
};
