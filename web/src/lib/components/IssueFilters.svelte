<script lang="ts">
	import type { Location } from '#lib/api/client.ts';
	import type { IssueFilterState, IssueSeverity, IssueStatus } from '#lib/issues.ts';
	import { ISSUE_SEVERITIES, ISSUE_STATUSES } from '#lib/issues.ts';
	import { msg } from '#lib/msg.ts';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import * as m from '#lib/paraglide/messages.js';

	interface Props {
		locations: Location[];
		filters: IssueFilterState;
	}

	let { locations, filters }: Props = $props();

	function updateParams(mutate: (params: URLSearchParams) => void) {
		const url = new URL(page.url.href);
		mutate(url.searchParams);
		// SvelteKit 3 replaced `noScroll`/`keepFocus` with `reset: false`.
		void goto(url, { reset: false });
	}

	function clearFilters() {
		updateParams((params) => {
			params.delete('status');
			params.delete('severity');
			params.delete('location');
			params.delete('closed');
		});
	}

	function clearLocations() {
		updateParams((params) => params.delete('location'));
	}

	function toggleLocation(id: string) {
		const next = filters.locationIds.includes(id)
			? filters.locationIds.filter((locationId) => locationId !== id)
			: [...filters.locationIds, id];

		updateParams((params) => {
			if (next.length === 0) params.delete('location');
			else params.set('location', next.join(','));
		});
	}

	function toggleStatus(status: IssueStatus) {
		const isSelected = filters.statuses.includes(status);
		// Never allow the filter to become empty - that would silently mean "all statuses".
		if (isSelected && filters.statuses.length <= 1) return;

		const next = isSelected
			? filters.statuses.filter((value) => value !== status)
			: [...filters.statuses, status];

		updateParams((params) => {
			if (next.length === 0) params.delete('status');
			else params.set('status', next.join(','));
		});
	}

	function toggleSeverity(severity: IssueSeverity) {
		const next = filters.severities.includes(severity)
			? filters.severities.filter((value) => value !== severity)
			: [...filters.severities, severity];

		updateParams((params) => {
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

<details class="mb-4 rounded border border-neutral-200 bg-white text-sm print:hidden">
	<summary class="flex w-full cursor-pointer items-start gap-3 p-3">
		<span class="shrink-0 text-xs font-semibold uppercase tracking-wide text-neutral-500">
			{m.page_issues_filters_heading()}
		</span>
		<span class="min-w-0 flex-1 text-xs text-neutral-500">
			<span>{m.page_issues_filter_locations()}: {locationSummary}</span>
			<span class="mx-1">·</span>
			<span>{m.page_issues_filter_status()}: {statusSummary}</span>
			<span class="mx-1">·</span>
			<span>{m.page_issues_filter_severity()}: {severitySummary}</span>
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
								onchange={() => toggleStatus(status)}
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
