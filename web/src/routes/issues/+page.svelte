<script lang="ts">
	import IssueCard from '#lib/components/IssueCard.svelte';
	import IssueFilters from '#lib/components/IssueFilters.svelte';
	import IssuePrintList from '#lib/components/IssuePrintList.svelte';
	import type { PageData } from './$types';
	import { browser } from '$app/env';
	import { beforeNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { downloadTextFile, sanitizeFileName, toCsv } from '#lib/download.ts';
	import { currentIssueUrl, resetIssueUrlQueue, updateIssueUrl } from '#lib/issueUrl.ts';
	import { parseIssueCompactMode, parseIssueSort, sortIssues } from '#lib/issues.ts';
	import { msg } from '#lib/msg.ts';
	import type { Issue } from '#lib/api/client.ts';
	import * as m from '#lib/paraglide/messages.js';

	let { data }: { data: PageData } = $props();

	let issueUrl = $derived(currentIssueUrl());
	let compactMode = $derived(parseIssueCompactMode(issueUrl.searchParams));
	let sort = $derived(parseIssueSort(issueUrl.searchParams.get('sort')));

	onMount(() => {
		// Drop any queued URL updates left over from an earlier visit.
		resetIssueUrlQueue();
	});

	beforeNavigate((navigation) => {
		// Our own gotos stay on /issues; other gotos (cart, session redirect)
		// must cancel queued issue URL updates.
		if (navigation.type === 'goto' && navigation.to?.url.pathname === '/issues') return;
		resetIssueUrlQueue();
	});

	let exportFileBase = $derived.by(() => {
		const selectedLocations = data.filters.locationIds
			.map((id) => data.locations.find((location) => location.id === id)?.name)
			.filter((name): name is string => Boolean(name));
		const locationPart =
			selectedLocations.length > 0
				? selectedLocations.join(', ')
				: m.page_issues_filter_all_locations();
		return sanitizeFileName(
			[page.data.user?.group_name, m.page_issues_print_title(), locationPart, data.dateStamp]
				.filter((part): part is string => Boolean(part))
				.join(' - ')
		);
	});

	$effect(() => {
		if (!browser) return;
		const previous = document.title;
		document.title = exportFileBase;
		return () => {
			document.title = previous;
		};
	});

	function formatExportDate(value: string) {
		return new Intl.DateTimeFormat(data.dateLocale, {
			dateStyle: 'short',
			timeStyle: 'short',
			timeZone: 'Europe/Stockholm'
		}).format(new Date(value));
	}

	function issueLocationNames(issue: Issue): string {
		const names = issue.articles.map((article) => article.location_name).filter(Boolean);
		return names.filter((name, index) => names.indexOf(name) === index).join(', ');
	}

	function issueAssigneeNames(issue: Issue): string {
		if (issue.assignees.length === 0) return m.page_issue_not_assigned();
		return issue.assignees.map((assignee) => assignee.user_name).join(', ');
	}

	function exportCsv() {
		const issues = sortIssues(data.printIssues, sort);
		const rows = [
			[
				m.page_issues_csv_title(),
				m.page_issues_filter_status(),
				m.page_issues_filter_severity(),
				m.page_issues_csv_locations(),
				m.page_issues_print_assigned(),
				m.page_issues_print_reporter(),
				m.page_issues_csv_created(),
				m.page_issues_csv_updated(),
				m.page_issue_description_heading()
			],
			...issues.map((issue) => [
				issue.title,
				msg(`issue_status_${issue.status}`) ?? issue.status,
				msg(`issue_severity_${issue.severity}`) ?? issue.severity,
				issueLocationNames(issue),
				issueAssigneeNames(issue),
				issue.reporter_name,
				formatExportDate(issue.created_at),
				formatExportDate(issue.updated_at),
				issue.description
			])
		];
		downloadTextFile(`${exportFileBase}.csv`, toCsv(rows), 'text/csv;charset=utf-8');
	}

	function toggleCompactMode() {
		void updateIssueUrl(
			(params) => {
				const isCompact = parseIssueCompactMode(params);
				params.delete('print');
				if (isCompact) params.delete('compact');
				else params.set('compact', 'true');
			},
			{ shallow: true }
		);
	}
</script>

<div class="mx-auto max-w-4xl p-4 print:max-w-none print:p-0">
	<div class="mb-4 flex flex-wrap items-center justify-between gap-2 print:hidden">
		<h1 class="text-heading-sm font-bold">{m.page_issues_heading()}</h1>
		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				onclick={toggleCompactMode}
				class="rounded border border-neutral-300 px-3 py-2 text-sm text-neutral-600 hover:bg-neutral-50"
			>
				{compactMode
					? m.page_issues_btn_close_print_mode()
					: m.page_issues_btn_print_mode()}
			</button>
			<a
				href="/issues/new"
				class="rounded bg-blue-700 px-4 py-2 text-sm text-white"
			>
				{m.page_issues_btn_report()}
			</a>
		</div>
	</div>

	<IssueFilters locations={data.locations} filters={data.filters} />

	<div class={compactMode ? 'hidden' : 'print:hidden'}>
		{#if data.myIssues.length > 0}
			<section class="mb-6">
				<h2 class="mb-2 text-sm font-medium text-neutral-500">{m.page_issues_my_issues()}</h2>
				<div class="space-y-2">
					{#each data.myIssues as issue (issue.id)}
						<IssueCard {issue} />
					{/each}
				</div>
			</section>
		{/if}

		{#if data.isManager && data.otherIssues.length > 0}
			<section class="mb-6">
				<h2 class="mb-2 text-sm font-medium text-neutral-500">{m.page_issues_other_issues()}</h2>
				<div class="space-y-2">
					{#each data.otherIssues as issue (issue.id)}
						<IssueCard {issue} />
					{/each}
				</div>
			</section>
		{/if}

		{#if data.myIssues.length === 0 && (!data.isManager || data.otherIssues.length === 0)}
			<p class="text-sm text-neutral-500">
				{data.hasAnyIssues ? m.page_issues_print_no_issues() : m.page_issues_empty()}
			</p>
		{/if}
	</div>

	<div class={compactMode ? '' : 'hidden print:block'}>
		<IssuePrintList
			issues={data.printIssues}
			locations={data.locations}
			hasAnyIssues={data.hasAnyIssues}
			groupName={page.data.user?.group_name ?? ''}
			filters={data.filters}
			today={data.today}
			dateLocale={data.dateLocale}
		/>
	</div>

	<div class="mt-6 flex flex-wrap gap-2 border-t border-neutral-200 pt-4 print:hidden">
		<button
			type="button"
			onclick={() => window.print()}
			class="rounded border border-neutral-300 px-3 py-2 text-sm text-neutral-600 hover:bg-neutral-50"
		>
			{m.page_issues_btn_print()}
		</button>
		<button
			type="button"
			onclick={exportCsv}
			class="rounded border border-neutral-300 px-3 py-2 text-sm text-neutral-600 hover:bg-neutral-50"
		>
			{m.page_issues_btn_export_csv()}
		</button>
	</div>
</div>
