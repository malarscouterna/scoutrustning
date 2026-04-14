import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch });
	const filter = url.searchParams.get('status') || 'reported_usable,reported_unusable,under_repair,lost';
	const showResolved = url.searchParams.get('resolved') === 'true';
	const effectiveFilter = showResolved ? filter + ',ok' : filter;

	const [myArticles, allArticles] = await Promise.all([
		api.listArticles({ status: effectiveFilter, mine: true }),
		api.listArticles({ status: effectiveFilter, mine: false })
	]);

	const myIds = new Set(myArticles.map(a => a.id));
	const otherArticles = allArticles.filter(a => !myIds.has(a.id));

	return { myArticles, otherArticles, filter, showResolved };
};
