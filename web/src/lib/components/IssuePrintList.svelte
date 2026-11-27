<script lang="ts">
	import type { Issue, Location } from '#lib/api/client.ts';
	import type {
		IssueFilterState,
		IssueGrouping,
		IssuePrintField,
		IssueSort
	} from '#lib/issues.ts';
	import {
		ISSUE_GROUPINGS,
		ISSUE_PRINT_FIELDS,
		ISSUE_SORTS,
		parseIssuePrintFields,
		sortIssues
	} from '#lib/issues.ts';
	import { updateIssueUrl } from '#lib/issueUrl.ts';
	import { msg } from '#lib/msg.ts';
	import { issueSeverityColors, issueStatusColors } from '#lib/styles.ts';
	import * as m from '#lib/paraglide/messages.js';

	interface Props {
		issues: Issue[];
		locations: Location[];
		grouping: IssueGrouping;
		sort: IssueSort;
		fields: IssuePrintField[];
		groupName: string;
		filters: IssueFilterState;
		today: string;
		dateLocale: string;
	}

	let { issues, locations, grouping, sort, fields, groupName, filters, today, dateLocale }: Props =
		$props();

	let checkedIssueIds = $state<string[]>([]);

	interface IssueGroup {
		key: string;
		label: string;
		issues: Issue[];
	}

	const UNKNOWN_LOCATION = '__unknown_location__';
	const UNASSIGNED = '__unassigned__';

	const severityOrder = ['missing', 'unusable', 'usable'];
	const statusOrder = ['open', 'in_progress', 'resolved', 'archived'];

	function updateParam(key: string, value: string) {
		void updateIssueUrl((params) => {
			if (value) params.set(key, value);
			else params.delete(key);
		});
	}

	function toggleField(field: IssuePrintField) {
		void updateIssueUrl(
			(params) => {
				const current = parseIssuePrintFields(params);
				const next = current.includes(field)
					? current.filter((value) => value !== field)
					: [...current, field];
				// Keep the param present even when empty: absent means defaults.
				params.set(
					'fields',
					ISSUE_PRINT_FIELDS.filter((value) => next.includes(value)).join(',')
				);
			},
			{ replace: true }
		);
	}

	function fieldLabel(field: IssuePrintField): string {
		switch (field) {
			case 'title':
				return m.page_issues_csv_title();
			case 'status':
				return m.page_issues_filter_status();
			case 'severity':
				return m.page_issues_filter_severity();
			case 'location':
				return m.page_issues_print_location();
			case 'assignee':
				return m.page_issues_print_assigned();
			case 'reporter':
				return m.page_issues_print_reporter();
			case 'date':
				return m.page_issues_print_field_date();
			case 'description':
				return m.page_issue_description_heading();
			case 'checkbox':
				return m.page_issues_print_field_checkbox();
			case 'id':
				return m.page_issues_print_field_id();
			default:
				return field;
		}
	}

	function toggleChecked(issueId: string) {
		checkedIssueIds = checkedIssueIds.includes(issueId)
			? checkedIssueIds.filter((id) => id !== issueId)
			: [...checkedIssueIds, issueId];
	}

	function addToGroup(groups: Map<string, IssueGroup>, key: string, label: string, issue: Issue) {
		let group = groups.get(key);
		if (!group) {
			group = { key, label, issues: [] };
			groups.set(key, group);
		}
		group.issues.push(issue);
	}

	let groups = $derived.by((): IssueGroup[] => {
		const sorted = sortIssues(issues, sort);

		if (grouping === 'severity') {
			const map = new Map<string, IssueGroup>();
			for (const issue of sorted) {
				const key = issue.severity;
				addToGroup(
					map,
					key,
					msg(`issue_severity_${key}`) ?? key,
					issue
				);
			}
			return [...map.values()].sort(
				(a, b) => severityOrder.indexOf(a.key) - severityOrder.indexOf(b.key)
			);
		}

		if (grouping === 'status') {
			const map = new Map<string, IssueGroup>();
			for (const issue of sorted) {
				const key = issue.status;
				addToGroup(
					map,
					key,
					msg(`issue_status_${key}`) ?? key,
					issue
				);
			}
			return [...map.values()].sort(
				(a, b) => statusOrder.indexOf(a.key) - statusOrder.indexOf(b.key)
			);
		}

		if (grouping === 'assignee') {
			const map = new Map<string, IssueGroup>();
			for (const issue of sorted) {
				if (issue.assignees.length === 0) {
					addToGroup(map, UNASSIGNED, m.page_issues_group_unassigned(), issue);
				} else {
					for (const assignee of issue.assignees) {
						addToGroup(map, assignee.user_id, assignee.user_name, issue);
					}
				}
			}
			return [...map.values()].sort((a, b) => {
				if (a.key === UNASSIGNED) return 1;
				if (b.key === UNASSIGNED) return -1;
				return a.label.localeCompare(b.label, dateLocale);
			});
		}

		// Default / grouping === 'location'
		const locationOrder = new Map(locations.map((location, index) => [location.id, index]));
		const map = new Map<string, IssueGroup>();
		for (const issue of sorted) {
			// When a location filter is active, group under the first matching article
			// so the issue appears under a location the user is filtering for; otherwise
			// (or when no article matches) fall back to the first article.
			const groupingArticle =
				(filters.locationIds.length > 0
					? issue.articles.find((article) =>
							filters.locationIds.includes(article.location_id)
						)
					: undefined) ?? issue.articles[0];
			const key = groupingArticle?.location_id ?? UNKNOWN_LOCATION;
			const label = groupingArticle
				? (locations.find((location) => location.id === groupingArticle.location_id)?.name ??
					groupingArticle.location_name ??
					m.page_issues_print_unknown_location())
				: m.page_issues_print_unknown_location();
			addToGroup(map, key, label, issue);
		}

		const orderOf = (key: string) => {
			if (key === UNKNOWN_LOCATION) return Number.MAX_SAFE_INTEGER;
			return locationOrder.get(key) ?? locations.length + 0.5;
		};

		return [...map.values()].sort((a, b) => orderOf(a.key) - orderOf(b.key));
	});

	let dateFormatter = $derived(
		new Intl.DateTimeFormat(dateLocale, {
			dateStyle: 'medium',
			timeZone: 'Europe/Stockholm'
		})
	);

	function formatDate(value: string): string {
		return dateFormatter.format(new Date(value));
	}

	function locationNames(issue: Issue): string[] {
		return [...new Set(issue.articles.map((article) => article.location_name).filter(Boolean))];
	}

	function assigneeNames(issue: Issue): string {
		if (issue.assignees.length === 0) return m.page_issue_not_assigned();
		return issue.assignees.map((assignee) => assignee.user_name).join(', ');
	}

	function metaItems(issue: Issue): string[] {
		const items: string[] = [];
		if (fields.includes('location')) {
			const names = locationNames(issue);
			items.push(
				`${m.page_issues_print_location()}: ${
					names.length > 0 ? names.join(', ') : m.page_issues_print_unknown_location()
				}`
			);
		}
		if (fields.includes('assignee')) {
			items.push(`${m.page_issues_print_assigned()}: ${assigneeNames(issue)}`);
		}
		if (fields.includes('reporter')) {
			items.push(`${m.page_issues_print_reporter()}: ${issue.reporter_name}`);
		}
		if (fields.includes('date')) {
			items.push(formatDate(issue.created_at));
		}
		return items;
	}

	let filterSummary = $derived.by(() => {
		const selectedLocations = filters.locationIds
			.map((id) => locations.find((location) => location.id === id)?.name)
			.filter((name): name is string => Boolean(name));
		const locationText =
			selectedLocations.length > 0
				? selectedLocations.join(', ')
				: m.page_issues_filter_all_locations();
		const statusText =
			filters.statuses.length > 0
				? filters.statuses.map((status) => msg(`issue_status_${status}`) ?? status).join(', ')
				: m.page_issues_filter_all();
		const severityText =
			filters.severities.length > 0
				? filters.severities.map((severity) => msg(`issue_severity_${severity}`) ?? severity).join(', ')
				: m.page_issues_filter_all();

		const parts = [
			`${m.page_issues_filter_locations()}: ${locationText}`,
			`${m.page_issues_filter_status()}: ${statusText}`,
			`${m.page_issues_filter_severity()}: ${severityText}`
		];
		if (filters.search) {
			parts.push(m.page_issues_filter_search_summary({ query: filters.search }));
		}
		return parts.join(' · ');
	});
</script>

<header class="mb-4 border-b border-neutral-300 pb-3">
	<div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
		<div>
			{#if groupName}
				<p class="text-xs uppercase tracking-wide text-neutral-500">{groupName}</p>
			{/if}
			<h1 class="text-lg font-bold">{m.page_issues_print_title()}</h1>
		</div>
		<p class="text-xs text-neutral-600">
			{today} · {m.page_issues_print_count({ count: issues.length })}
		</p>
	</div>
	<p class="mt-1 text-[11px] leading-snug text-neutral-600">{filterSummary}</p>
</header>

<div class="mb-4 flex flex-wrap items-center gap-3 text-sm print:hidden">
	<label class="flex items-center gap-2">
		<span class="text-xs font-medium text-neutral-500">{m.page_issues_print_grouping()}</span>
		<select
			class="rounded border border-neutral-300 bg-white px-2 py-1 text-sm"
			value={grouping}
			onchange={(event) => updateParam('group', event.currentTarget.value)}
		>
			{#each ISSUE_GROUPINGS as value (value)}
				<option value={value}>
					{msg(`page_issues_group_${value}`) ?? value}
				</option>
			{/each}
		</select>
	</label>

	<label class="flex items-center gap-2">
		<span class="text-xs font-medium text-neutral-500">{m.page_issues_print_sorting()}</span>
		<select
			class="rounded border border-neutral-300 bg-white px-2 py-1 text-sm"
			value={sort}
			onchange={(event) => updateParam('sort', event.currentTarget.value)}
		>
			{#each ISSUE_SORTS as value (value)}
				<option value={value}>
					{msg(`page_issues_sort_${value}`) ?? value}
				</option>
			{/each}
		</select>
	</label>

	<details class="w-full">
		<summary class="cursor-pointer text-xs font-medium text-neutral-500">
			{m.page_issues_print_fields()}
		</summary>
		<div class="mt-2 grid grid-cols-2 gap-1.5 sm:grid-cols-3 lg:grid-cols-5">
			{#each ISSUE_PRINT_FIELDS as field (field)}
				<label class="flex items-center gap-2 text-xs">
					<input
						type="checkbox"
						class="h-3 w-3 accent-blue-700"
						checked={fields.includes(field)}
						onchange={() => toggleField(field)}
					/>
					<span>{fieldLabel(field)}</span>
				</label>
			{/each}
		</div>
	</details>
</div>

{#if issues.length === 0}
	<p class="text-sm text-neutral-500">{m.page_issues_print_no_issues()}</p>
{:else}
	<div class="space-y-5">
		{#each groups as group (group.key)}
			<section>
				<h2
					class="mb-2 flex items-baseline justify-between gap-3 border-b border-neutral-200 pb-1 text-sm font-semibold break-after-avoid"
				>
					<span>{group.label}</span>
					<span class="text-xs font-normal text-neutral-500">{group.issues.length}</span>
				</h2>

				<div class="space-y-3">
					{#each group.issues as issue (issue.id)}
						<article class="break-inside-avoid border-b border-neutral-200 pb-3 last:border-b-0">
							<div class="flex items-start gap-2">
								{#if fields.includes('checkbox')}
									<input
										type="checkbox"
										class="mt-0.5 h-4 w-4 shrink-0 accent-blue-700"
										checked={checkedIssueIds.includes(issue.id)}
										onchange={() => toggleChecked(issue.id)}
										aria-label={issue.title}
									/>
								{/if}

								<div class="min-w-0 flex-1">
									<div class="flex flex-wrap items-start justify-between gap-2">
										<div class="flex flex-wrap items-baseline gap-2">
											{#if fields.includes('id')}
												<span class="font-mono text-[11px] text-neutral-500">
													#{issue.id.slice(0, 6)}
												</span>
											{/if}
											{#if fields.includes('title')}
												<a
													href="/issues/{issue.id}"
													class="text-sm font-medium text-blue-800 hover:underline print:text-black print:no-underline"
												>
													{issue.title}
												</a>
											{/if}
										</div>

										{#if fields.includes('status') || fields.includes('severity')}
											<div class="flex flex-wrap gap-1">
												{#if fields.includes('status')}
													<span
														class="rounded px-2 py-0.5 text-[11px] font-medium {issueStatusColors[
															issue.status
														] ?? 'bg-neutral-100'} print:border print:border-neutral-400 print:bg-transparent print:text-black"
													>
														{msg(`issue_status_${issue.status}`) ?? issue.status}
													</span>
												{/if}
												{#if fields.includes('severity')}
													<span
														class="rounded px-2 py-0.5 text-[11px] font-medium {issueSeverityColors[
															issue.severity
														] ?? 'bg-neutral-100'} print:border print:border-neutral-400 print:bg-transparent print:text-black"
													>
														{msg(`issue_severity_${issue.severity}`) ?? issue.severity}
													</span>
												{/if}
											</div>
										{/if}
									</div>

									{#if metaItems(issue).length > 0}
										<p class="mt-1 text-xs text-neutral-600">
											{metaItems(issue).join(' · ')}
										</p>
									{/if}

									{#if fields.includes('description') && issue.description}
										<p class="mt-1 whitespace-pre-line break-words text-sm text-neutral-800">
											{issue.description}
										</p>
									{/if}
								</div>
							</div>
						</article>
					{/each}
				</div>
			</section>
		{/each}
	</div>
{/if}
