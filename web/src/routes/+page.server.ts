import { createApiClient } from '$lib/api/client';
import { isManager } from '$lib/user';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();

	const [bookings, myIssues] = await Promise.all([
		api.listBookings(),
		api.listArticles({ status: 'reported_usable,reported_unusable,under_repair,lost', mine: true })
	]);

	let managerIssues: Awaited<ReturnType<typeof api.listArticles>> = [];
	let pendingApprovals: Awaited<ReturnType<typeof api.listBookings>> = [];
	if (isManager(user)) {
		[managerIssues, pendingApprovals] = await Promise.all([
			api.listArticles({ status: 'reported_usable,reported_unusable,under_repair,lost' }),
			api.listPendingApprovals()
		]);
	}

	return { bookings, myIssues, managerIssues, pendingApprovals };
};
