import { createApiClient } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent, params }) => {
	const { user } = await parent();
	if (!user) throw redirect(302, '/');

	const api = createApiClient({ fetch });
	const article = await api.getArticle(params.id);
	const { events, has_more } = await api.listArticleEvents(params.id, 10);

	return { article, events, has_more };
};
