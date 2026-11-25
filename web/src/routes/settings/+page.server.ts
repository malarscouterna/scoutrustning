import { createApiClient, type BrokenArticleLink } from '#lib/api/client.ts';
import { isManager } from '#lib/user.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, parent }) => {
	const { user } = await parent();
	if (!user) return { locations: [], categories: [], groupSettings: null, notificationPrefs: null };

	const api = createApiClient({ fetch });
	const mgr = isManager(user);

	const [locations, categories, teams, notifResult] = await Promise.all([
		api.listLocations(),
		api.listCategories(),
		mgr ? api.listTeams() : Promise.resolve([]),
		api.getNotificationPrefs().catch(() => null)
	]);

	let groupSettings = null;
	let brokenLinks: BrokenArticleLink[] = [];
	if (mgr) {
		try {
			groupSettings = await api.getGroupSettings();
		} catch { /* defaults */ }
		brokenLinks = await api.listBrokenArticleLinks().catch(() => []);
	}

	return { locations, categories, teams, groupSettings, brokenLinks, notificationPrefs: notifResult?.prefs ?? null };
};
