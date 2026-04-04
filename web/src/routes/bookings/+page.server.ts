import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
	const api = createApiClient({ fetch });
	const bookings = await api.listBookings();
	return { bookings };
};
