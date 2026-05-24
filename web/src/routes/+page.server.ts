import { createApiClient } from '$lib/api/client';
import { isManager } from '$lib/user';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();

	if (!user) return { bookings: [], myIssues: [], managerIssues: [], pendingApprovals: [] };

	const [bookings, myIssues] = await Promise.all([
		api.listBookings(),
		api.listIssues({ mine: true, status: 'open,in_progress' })
	]);

	let managerIssues: Awaited<ReturnType<typeof api.listIssues>> = [];
	let pendingApprovals: Awaited<ReturnType<typeof api.listBookings>> = [];
	if (isManager(user)) {
		[managerIssues, pendingApprovals] = await Promise.all([
			api.listIssues({ status: 'open,in_progress' }),
			api.listPendingApprovals()
		]);
	}

	return { bookings, myIssues, managerIssues, pendingApprovals };
};
