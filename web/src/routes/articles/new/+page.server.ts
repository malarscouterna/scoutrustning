import { isManager } from '$lib/user';
import { createApiClient } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent, url }) => {
	const { user } = await parent();
	if (!isManager(user)) throw redirect(302, '/browse');

	const api = createApiClient({ fetch });
	const [locations, categories] = await Promise.all([
		api.listLocations(),
		api.listCategories()
	]);

	// Pre-fill from existing article group
	let prefill = null;
	const fromName = url.searchParams.get('from');
	const fromLocation = url.searchParams.get('location');
	if (fromName) {
		const articles = await api.listArticles({ search: fromName });
		const match = articles.find(a =>
			a.commercial_name === fromName &&
			(!fromLocation || a.location_id === fromLocation)
		);
		if (match) {
			prefill = {
				commercial_name: match.commercial_name,
				category_id: match.category_id,
				location_id: match.location_id,
				individually_tracked: match.individually_tracked,
				approval_level: match.approval_level,
				description: match.description,
				instructions: match.instructions,
				place: match.place
			};
		}
	}

	return { locations, categories, prefill };
};
