import { createApiClient } from '#lib/api/client.ts';
import { isManager } from '#lib/user.ts';
import { parseIssueFilters, parseIssueGrouping, parseIssueSort } from '#lib/issues.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url, parent }) => {
	const api = createApiClient({ fetch });
	const { user } = await parent();
	const filters = parseIssueFilters(url.searchParams);
	const printMode = url.searchParams.get('print') === 'true';
	const grouping = parseIssueGrouping(url.searchParams.get('group'));
	const sort = parseIssueSort(url.searchParams.get('sort'));

	const status = filters.statuses.join(',') || undefined;
	const severity = filters.severities.join(',') || undefined;

	const [myIssues, allIssues, locations] = await Promise.all([
		api.listIssues({
			status,
			severity,
			location_ids: filters.locationIds,
			mine: true
		}),
		isManager(user)
			? api.listIssues({
					status,
					severity,
					location_ids: filters.locationIds
				})
			: Promise.resolve([]),
		api.listLocations()
	]);

	const myIds = new Set(myIssues.map((i) => i.id));
	const otherIssues = allIssues.filter((i) => !myIds.has(i.id));
	const printIssues = isManager(user) ? allIssues : myIssues;
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
		printMode,
		grouping,
		sort,
		today,
		dateStamp,
		dateLocale,
		isManager: isManager(user)
	};
};
