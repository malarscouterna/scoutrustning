import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch });
	const bookingId = url.searchParams.get('id');

	const [locations, categories, teams] = await Promise.all([
		api.listLocations(),
		api.listCategories(),
		api.listTeams()
	]);

	if (bookingId) {
		const { booking, items } = await api.getBooking(bookingId);
		return { locations, categories, teams, existing: { booking, items } };
	}

	return { locations, categories, teams, existing: null };
};
