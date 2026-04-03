import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
	const api = createApiClient({ fetch, persona: 'leader-yggdrasil' });
	const bookings = await api.listBookings();
	return { bookings };
};
