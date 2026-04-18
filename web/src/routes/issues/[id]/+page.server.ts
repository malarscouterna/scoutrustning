import { createApiClient } from '$lib/api/client';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const api = createApiClient({ fetch });
	try {
		const issue = await api.getIssue(params.id);
		return { issue };
	} catch {
		error(404, 'Ärendet hittades inte');
	}
};
