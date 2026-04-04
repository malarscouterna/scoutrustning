import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch });
	const filter = url.searchParams.get('status') || 'reported_usable,reported_unusable,under_repair,lost';
	const articles = await api.listArticles({ status: filter });
	return { articles, filter };
};
