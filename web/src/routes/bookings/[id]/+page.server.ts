import { createApiClient } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const api = createApiClient({ fetch, persona: 'leader-yggdrasil' });
	const result = await api.getBooking(params.id);
	return result;
};
