import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();
	const bookings = await api.listBookings();

	let pendingCount = 0;
	if (user?.roles.includes('equipment_manager')) {
		pendingCount = bookings.filter(b => b.status === 'submitted').length;
	}

	return { bookings, pendingCount, user };
};
