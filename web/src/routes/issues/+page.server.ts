import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url, parent }) => {
	const { user } = await parent();
	const api = createApiClient({ fetch });
	const filter = url.searchParams.get('status') || 'reported_usable,reported_unusable,under_repair,lost';
	const isManager = user?.roles?.includes('equipment_manager') ?? false;
	const mineParam = url.searchParams.get('mine');
	const mine = mineParam !== null ? mineParam !== 'false' : !isManager;
	const articles = await api.listArticles({ status: filter, mine });
	return { articles, filter, mine };
};
