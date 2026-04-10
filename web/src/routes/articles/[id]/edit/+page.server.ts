import { createApiClient, type Article } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent, params, url }) => {
	const { user } = await parent();
	if (!user?.roles.includes('equipment_manager')) throw redirect(302, '/browse');

	const api = createApiClient({ fetch });
	const [article, locations, categories] = await Promise.all([
		api.getArticle(params.id),
		api.listLocations(),
		api.listCategories()
	]);

	let groupCount: number | null = null;
	let groupItems: Article[] | null = null;
	if (url.searchParams.get('group') === 'true') {
		const all = await api.listArticles({
			search: article.commercial_name,
			location_id: article.location_id,
			status: 'ok,reported_usable,incoming,reported_unusable,under_repair,lost'
		});
		const siblings = all.filter(
			a => a.commercial_name === article.commercial_name && a.location_id === article.location_id
		);
		groupCount = siblings.length;
		groupItems = siblings;
	}

	return { article, locations, categories, groupCount, groupItems };
};
