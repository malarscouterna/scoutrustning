import { createApiClient } from '#lib/api/client.ts';
import { isManager } from '#lib/user.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();

	if (!user) return { bookings: [], myIssues: [], managerIssues: [], pendingApprovals: [] };

	const manager = isManager(user);
	const [bookings, issues, pendingApprovals] = await Promise.all([
		api.listBookings(),
		manager
			? api.listIssues({ status: 'open,in_progress', include_mine: true })
			: api.listIssues({ mine: true, status: 'open,in_progress' }),
		manager ? api.listPendingApprovals() : Promise.resolve([])
	]);

	const myIssues = manager ? issues.filter((issue) => issue.is_mine) : issues;
	const managerIssues = manager ? issues.filter((issue) => !issue.is_mine) : [];

	return { bookings, myIssues, managerIssues, pendingApprovals };
};
