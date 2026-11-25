import { createApiClient } from '#lib/api/client.ts';
import { isManager } from '#lib/user.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();
	const bookings = await api.listBookings();

	let pendingCount = 0;
	if (isManager(user)) {
		pendingCount = bookings.filter(b => b.status === 'submitted').length;
	}

	return { bookings, pendingCount, user };
};
