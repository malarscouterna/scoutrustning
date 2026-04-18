import { createApiClient } from '$lib/api/client';
import { isManager } from '$lib/user';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();
	const showClosed = url.searchParams.get('closed') === 'true';

	const activeStatuses = 'open,in_progress';
	const allStatuses = showClosed ? 'open,in_progress,resolved,archived' : activeStatuses;

	const [myIssues, allIssues] = await Promise.all([
		api.listIssues({ status: allStatuses, mine: true }),
		isManager(user) ? api.listIssues({ status: allStatuses }) : Promise.resolve([])
	]);

	const myIds = new Set(myIssues.map(i => i.id));
	const otherIssues = allIssues.filter(i => !myIds.has(i.id));

	return { myIssues, otherIssues, showClosed, isManager: isManager(user) };
};
