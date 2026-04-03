import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch, persona: 'leader-yggdrasil' });
	const bookingId = url.searchParams.get('id');

	const [locations, categories, units] = await Promise.all([
		api.listLocations(),
		api.listCategories(),
		api.listUnits()
	]);

	if (bookingId) {
		const { booking, items } = await api.getBooking(bookingId);
		return { locations, categories, units, existing: { booking, items } };
	}

	return { locations, categories, units, existing: null };
};
