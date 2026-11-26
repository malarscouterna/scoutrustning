<script lang="ts">
	import IssueCard from '#lib/components/IssueCard.svelte';
	import IssueFilters from '#lib/components/IssueFilters.svelte';
	import IssuePrintList from '#lib/components/IssuePrintList.svelte';
	import type { PageData } from './$types';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import * as m from '#lib/paraglide/messages.js';

	let { data }: { data: PageData } = $props();

	function togglePrintMode() {
		const url = new URL(page.url.href);
		if (data.printMode) {
			url.searchParams.delete('print');
		} else {
			url.searchParams.set('print', 'true');
		}
		// SvelteKit 3 replaced `noScroll`/`keepFocus` with `reset: false`.
		void goto(url, { reset: false });
	}
</script>

<div class="mx-auto max-w-4xl p-4 print:max-w-none print:p-0">
	<div class="mb-4 flex flex-wrap items-center justify-between gap-2 print:hidden">
		<h1 class="text-heading-sm font-bold">{m.page_issues_heading()}</h1>
		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				onclick={togglePrintMode}
				class="rounded border border-neutral-300 px-3 py-2 text-sm text-neutral-600 hover:bg-neutral-50"
			>
				{data.printMode
					? m.page_issues_btn_close_print_mode()
					: m.page_issues_btn_print_mode()}
			</button>
			{#if data.printMode}
				<button
					type="button"
					onclick={() => window.print()}
					class="rounded bg-blue-700 px-4 py-2 text-sm text-white print:hidden"
				>
					{m.page_issues_btn_print()}
				</button>
			{/if}
			<a
				href="/issues/new"
				class="rounded bg-blue-700 px-4 py-2 text-sm text-white"
			>
				{m.page_issues_btn_report()}
			</a>
		</div>
	</div>

	<IssueFilters locations={data.locations} filters={data.filters} />

	<div class={data.printMode ? 'hidden' : 'print:hidden'}>
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
			<p class="text-sm text-neutral-500">{m.page_issues_empty()}</p>
		{/if}
	</div>

	<div class={data.printMode ? '' : 'hidden print:block'}>
		<IssuePrintList
			issues={data.printIssues}
			locations={data.locations}
			grouping={data.grouping}
			sort={data.sort}
			groupName={page.data.user?.group_name ?? ''}
			filters={data.filters}
			today={data.today}
			dateLocale={data.dateLocale}
		/>
	</div>
</div>
