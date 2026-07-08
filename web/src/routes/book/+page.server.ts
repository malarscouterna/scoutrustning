import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = createApiClient({ fetch });
	const bookingId = url.searchParams.get('id');

	const teams = await api.listTeams();

	if (bookingId) {
		const { booking, items, archive_deadline } = await api.getBooking(bookingId);
		return { teams, existing: { booking, items, archive_deadline } };
	}

	return { teams, existing: null };
};
