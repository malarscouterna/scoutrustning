import { isManager } from '$lib/user';
import { createApiClient, type Article } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent, params, url }) => {
	const { user } = await parent();
	if (!isManager(user)) throw redirect(302, '/browse');

	const api = createApiClient({ fetch });
	const [article, locations, categories, links] = await Promise.all([
		api.getArticle(params.id),
		api.listLocations(),
		api.listCategories(),
		// The visible links on the article hide trusted links from lower access
		// levels; the editor must see (and round-trip) all of them.
		api.listArticleLinks(params.id)
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

	return { article: { ...article, links }, locations, categories, groupCount, groupItems };
};
