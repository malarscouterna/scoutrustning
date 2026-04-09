import { createApiClient } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent, params }) => {
	const { user } = await parent();
	if (!user) throw redirect(302, '/');

	const api = createApiClient({ fetch });
	const article = await api.getArticle(params.id);

	// For quantity tracked: fetch group siblings for overview (status summary, purchase info)
	let groupArticles: Awaited<ReturnType<typeof api.listArticles>> | null = null;
	if (!article.individually_tracked) {
		const all = await api.listArticles({
			search: article.commercial_name,
			location_id: article.location_id,
			status: 'ok,reported_usable,incoming,reported_unusable,under_repair,lost,archived'
		});
		// Filter to exact commercial_name match (search is ILIKE)
		groupArticles = all.filter(
			a => a.commercial_name === article.commercial_name && a.location_id === article.location_id
		);
	}

	return { article, groupArticles };
};
