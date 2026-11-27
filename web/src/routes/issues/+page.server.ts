import { createApiClient } from '#lib/api/client.ts';
import { isManager } from '#lib/user.ts';
import { parseIssueFilters } from '#lib/issues.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();
	const filters = parseIssueFilters(url.searchParams);
	const manager = isManager(user);

	const status = filters.statuses.join(',') || undefined;
	const severity = filters.severities.join(',') || undefined;
	const search = filters.search || undefined;

	// Managers need the full group list plus a flag for their own issues. Fetch
	// it once instead of running the same filtered query twice.
	const [issues, locations] = await Promise.all([
		manager
			? api.listIssues({
					status,
					severity,
					location_ids: filters.locationIds,
					search,
					include_mine: true
				})
			: api.listIssues({
					status,
					severity,
					location_ids: filters.locationIds,
					search,
					mine: true
				}),
		api.listLocations()
	]);

	const myIssues = manager ? issues.filter((issue) => issue.is_mine) : issues;
	const otherIssues = manager ? issues.filter((issue) => !issue.is_mine) : [];
	const printIssues = issues;
	// Only when the filtered result is empty do we need to know whether the user
	// has no issues at all or simply none matching the current filters.
	const hasAnyIssues =
		issues.length > 0 || (await api.hasIssues({ mine: !manager })).exists;
	const dateLocale = user?.language === 'en' ? 'en-GB' : 'sv-SE';
	const today = new Intl.DateTimeFormat(dateLocale, {
		dateStyle: 'long',
		timeZone: 'Europe/Stockholm'
	}).format(new Date());
	const dateStamp = new Intl.DateTimeFormat('sv-SE', {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		timeZone: 'Europe/Stockholm'
	}).format(new Date());

	return {
		myIssues,
		otherIssues,
		printIssues,
		locations,
		filters,
		hasAnyIssues,
		today,
		dateStamp,
		dateLocale,
		isManager: manager
	};
};
