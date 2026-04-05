import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params, parent }) => {
	const api = createApiClient({ fetch });
	const result = await api.getBooking(params.id);
	const { user } = await parent();
	return { ...result, user };
};
