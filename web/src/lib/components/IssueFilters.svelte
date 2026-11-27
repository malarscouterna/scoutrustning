<script lang="ts">
	import type { Location } from '#lib/api/client.ts';
	import type { IssueFilterState, IssuePrintField, IssueSeverity, IssueStatus } from '#lib/issues.ts';
	import {
		ISSUE_GROUPINGS,
		ISSUE_PRINT_FIELDS,
		ISSUE_SEVERITIES,
		ISSUE_SORTS,
		ISSUE_STATUSES,
		parseIssueCompactMode,
		parseIssueFilters,
		parseIssueGrouping,
		parseIssuePrintFields,
		parseIssueSort
	} from '#lib/issues.ts';
	import { currentIssueUrl, updateIssueUrl } from '#lib/issueUrl.ts';
	import { msg } from '#lib/msg.ts';
	import { afterNavigate, beforeNavigate } from '$app/navigation';
	import { onDestroy, untrack } from 'svelte';
	import * as m from '#lib/paraglide/messages.js';

	interface Props {
		locations: Location[];
		filters: IssueFilterState;
	}

	let { locations, filters }: Props = $props();

	let issueUrl = $derived(currentIssueUrl());
	let compactMode = $derived(parseIssueCompactMode(issueUrl.searchParams));
	let grouping = $derived(parseIssueGrouping(issueUrl.searchParams.get('group')));
	let sort = $derived(parseIssueSort(issueUrl.searchParams.get('sort')));
	let fields = $derived(parseIssuePrintFields(issueUrl.searchParams));

	let search = $state(untrack(() => filters.search));
	let pendingSearch: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout> | undefined;

	$effect(() => {
		const fromUrl = filters.search;
		untrack(() => {
			if (pendingSearch !== null) {
				if (fromUrl === pendingSearch) pendingSearch = null;
				return;
			}
			if (search !== fromUrl) search = fromUrl;
		});
	});

	onDestroy(() => {
		if (searchTimer) clearTimeout(searchTimer);
	});

	beforeNavigate((navigation) => {
		if (navigation.type === 'goto' && navigation.to?.url.pathname === '/issues') return;
		// Cancel a pending debounce so it cannot re-apply a search after the user
		// navigates away or presses Back/Forward.
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = undefined;
		pendingSearch = null;
	});

	afterNavigate((navigation) => {
		if (navigation.type !== 'popstate') return;
		// Cancel any pending debounce and resync the input with the history target.
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = undefined;
		pendingSearch = null;
		search = navigation.to?.url.searchParams.get('search')?.trim() ?? '';
	});

	function updateParams(
		mutate: (params: URLSearchParams) => void,
		options?: { replace?: boolean }
	) {
		return updateIssueUrl(mutate, options);
	}

	function setGrouping(value: string) {
		void updateIssueUrl(
			(params) => {
				if (value === 'location') params.delete('group');
				else params.set('group', value);
			},
			{ shallow: true }
		);
	}

	function setSort(value: string) {
		void updateIssueUrl(
			(params) => {
				if (value === 'oldest') params.delete('sort');
				else params.set('sort', value);
			},
			{ shallow: true }
		);
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
			{ replace: true, shallow: true }
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

	async function applySearch(value: string) {
		try {
			await updateParams(
				(params) => {
					if (value) params.set('search', value);
					else params.delete('search');
				},
				{ replace: true }
			);
		} finally {
			if (pendingSearch === value) pendingSearch = null;
		}
	}

	function scheduleSearch(value: string) {
		search = value;
		pendingSearch = value;
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			searchTimer = undefined;
			void applySearch(value);
		}, 300);
	}

	function flushSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = undefined;
		pendingSearch = search;
		void applySearch(search);
	}

	function handleSearchKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter') return;
		event.preventDefault();
		flushSearch();
	}

	function clearFilters() {
		if (searchTimer) {
			clearTimeout(searchTimer);
			searchTimer = undefined;
		}
		search = '';
		pendingSearch = null;
		updateParams((params) => {
			params.delete('status');
			params.delete('severity');
			params.delete('location');
			params.delete('closed');
			params.delete('search');
		});
	}

	function clearLocations() {
		updateParams((params) => params.delete('location'));
	}

	function toggleLocation(id: string) {
		updateParams((params) => {
			const current = parseIssueFilters(params).locationIds;
			const next = current.includes(id)
				? current.filter((locationId) => locationId !== id)
				: [...current, id];

			if (next.length === 0) params.delete('location');
			else params.set('location', next.join(','));
		});
	}

	function toggleStatus(status: IssueStatus, input: HTMLInputElement) {
		let blocked = false;
		updateParams((params) => {
			const current = parseIssueFilters(params).statuses;
			const isSelected = current.includes(status);
			// Never allow the filter to become empty - that would silently mean "all statuses".
			if (isSelected && current.length <= 1) {
				blocked = true;
				return;
			}

			const next = isSelected
				? current.filter((value) => value !== status)
				: [...current, status];

			if (next.length === 0) params.delete('status');
			else params.set('status', next.join(','));
		});
		if (blocked) input.checked = true;
	}

	function toggleSeverity(severity: IssueSeverity) {
		updateParams((params) => {
			const current = parseIssueFilters(params).severities;
			const next = current.includes(severity)
				? current.filter((value) => value !== severity)
				: [...current, severity];

			if (next.length === 0) params.delete('severity');
			else params.set('severity', next.join(','));
		});
	}

	function joinLabels(keys: string[], prefix: string): string {
		return keys.map((key) => msg(`${prefix}_${key}`) ?? key).join(', ');
	}

	let locationSummary = $derived.by(() => {
		if (filters.locationIds.length === 0) return m.page_issues_filter_all_locations();
		const names = filters.locationIds
			.map((id) => locations.find((location) => location.id === id)?.name)
			.filter((name): name is string => Boolean(name));
		return names.length > 0 ? names.join(', ') : m.page_issues_filter_all_locations();
	});

	let statusSummary = $derived(
		filters.statuses.length > 0
			? joinLabels(filters.statuses, 'issue_status')
			: m.page_issues_filter_all()
	);

	let severitySummary = $derived(
		filters.severities.length > 0
			? joinLabels(filters.severities, 'issue_severity')
			: m.page_issues_filter_all()
	);

	const chipBase =
		'inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-3 py-1 text-xs';
	const chipSelected = 'border-blue-300 bg-blue-50 text-blue-800';
	const chipIdle = 'border-neutral-300 text-neutral-600 hover:bg-neutral-50';
</script>

<div class="mb-3 print:hidden">
	<input
		type="search"
		class="w-full rounded border border-neutral-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none"
		placeholder={m.page_issues_filter_search_placeholder()}
		aria-label={m.page_issues_filter_search()}
		value={search}
		oninput={(event) => scheduleSearch(event.currentTarget.value)}
		onkeydown={handleSearchKeydown}
	/>
</div>

<details class="mb-4 rounded border border-neutral-200 bg-white text-sm print:hidden">
	<summary class="flex w-full cursor-pointer items-start gap-3 p-3">
		<span class="shrink-0 text-xs font-semibold uppercase tracking-wide text-neutral-500">
			{compactMode
				? m.page_issues_filters_heading_compact()
				: m.page_issues_filters_heading()}
		</span>
		<span class="min-w-0 flex-1 text-xs text-neutral-500">
			<span>{m.page_issues_filter_locations()}: {locationSummary}</span>
			<span class="mx-1">·</span>
			<span>{m.page_issues_filter_status()}: {statusSummary}</span>
			<span class="mx-1">·</span>
			<span>{m.page_issues_filter_severity()}: {severitySummary}</span>
			{#if filters.search}
				<span class="mx-1">·</span>
				<span>{m.page_issues_filter_search_summary({ query: filters.search })}</span>
			{/if}
			{#if compactMode && (grouping !== 'location' || sort !== 'oldest')}
				<span class="mx-1">·</span>
				<span>
					{m.page_issues_filter_view()}:
					{msg(`page_issues_group_${grouping}`) ?? grouping},
					{msg(`page_issues_sort_${sort}`) ?? sort}
				</span>
			{/if}
		</span>
		<span class="chevron mt-0.5 shrink-0 text-xs text-neutral-400 transition-transform" aria-hidden="true">▾</span>
	</summary>

	<div class="border-t border-neutral-100 p-3">
		<div class="mb-3 flex justify-end">
			<button
				type="button"
				onclick={clearFilters}
				class="text-xs text-blue-700 hover:underline"
			>
				{m.page_issues_filter_clear()}
			</button>
		</div>

		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<fieldset class="min-w-0">
				<legend class="mb-2 text-xs font-medium text-neutral-500">
					{m.page_issues_filter_locations()}
				</legend>
				<div class="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto pr-1">
					<button
						type="button"
						onclick={clearLocations}
						aria-pressed={filters.locationIds.length === 0}
						class="{chipBase} {filters.locationIds.length === 0 ? chipSelected : chipIdle}"
					>
						{m.page_issues_filter_all_locations()}
					</button>
					{#each locations as location (location.id)}
						<label
							class="{chipBase} {filters.locationIds.includes(location.id)
								? chipSelected
								: chipIdle}"
						>
							<input
								type="checkbox"
								class="h-3 w-3 accent-blue-700"
								checked={filters.locationIds.includes(location.id)}
								onchange={() => toggleLocation(location.id)}
							/>
							<span>{location.name}</span>
						</label>
					{/each}
				</div>
			</fieldset>

			<fieldset class="min-w-0">
				<legend class="mb-2 text-xs font-medium text-neutral-500">
					{m.page_issues_filter_status()}
				</legend>
				<div class="flex flex-wrap gap-1.5">
					{#each ISSUE_STATUSES as status (status)}
						<label
							class="{chipBase} {filters.statuses.includes(status) ? chipSelected : chipIdle}"
						>
							<input
								type="checkbox"
								class="h-3 w-3 accent-blue-700"
								checked={filters.statuses.includes(status)}
								disabled={filters.statuses.length <= 1 && filters.statuses.includes(status)}
								title={m.page_issues_filter_status_required()}
								onchange={(event) => toggleStatus(status, event.currentTarget)}
							/>
							<span>{msg(`issue_status_${status}`) ?? status}</span>
						</label>
					{/each}
				</div>
			</fieldset>

			<fieldset class="min-w-0">
				<legend class="mb-2 text-xs font-medium text-neutral-500">
					{m.page_issues_filter_severity()}
				</legend>
				<div class="flex flex-wrap gap-1.5">
					{#each ISSUE_SEVERITIES as severity (severity)}
						<label
							class="{chipBase} {filters.severities.includes(severity)
								? chipSelected
								: chipIdle}"
						>
							<input
								type="checkbox"
								class="h-3 w-3 accent-blue-700"
								checked={filters.severities.includes(severity)}
								onchange={() => toggleSeverity(severity)}
							/>
							<span>{msg(`issue_severity_${severity}`) ?? severity}</span>
						</label>
					{/each}
				</div>
			</fieldset>

			{#if compactMode}
				<fieldset class="min-w-0 sm:col-span-2 lg:col-span-3">
					<legend class="mb-2 text-xs font-medium text-neutral-500">
						{m.page_issues_filter_view()}
					</legend>
					<div class="flex flex-wrap items-start gap-x-5 gap-y-3">
						<label class="flex items-center gap-2">
							<span class="text-xs font-medium text-neutral-500">
								{m.page_issues_print_grouping()}
							</span>
							<select
								class="rounded border border-neutral-300 bg-white px-2 py-1 text-sm"
								value={grouping}
								onchange={(event) => setGrouping(event.currentTarget.value)}
							>
								{#each ISSUE_GROUPINGS as value (value)}
									<option value={value}>
										{msg(`page_issues_group_${value}`) ?? value}
									</option>
								{/each}
							</select>
						</label>

						<label class="flex items-center gap-2">
							<span class="text-xs font-medium text-neutral-500">
								{m.page_issues_print_sorting()}
							</span>
							<select
								class="rounded border border-neutral-300 bg-white px-2 py-1 text-sm"
								value={sort}
								onchange={(event) => setSort(event.currentTarget.value)}
							>
								{#each ISSUE_SORTS as value (value)}
									<option value={value}>
										{msg(`page_issues_sort_${value}`) ?? value}
									</option>
								{/each}
							</select>
						</label>

						<div class="min-w-0">
							<span class="mb-1 block text-xs font-medium text-neutral-500">
								{m.page_issues_print_fields()}
							</span>
							<div class="flex flex-wrap gap-x-4 gap-y-1">
								{#each ISSUE_PRINT_FIELDS as field (field)}
									<label class="flex items-center gap-1.5 text-xs">
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
						</div>
					</div>
				</fieldset>
			{/if}
		</div>
	</div>
</details>

<style>
	summary {
		list-style: none;
	}

	summary::-webkit-details-marker {
		display: none;
	}

	details[open] .chevron {
		transform: rotate(180deg);
	}
</style>
