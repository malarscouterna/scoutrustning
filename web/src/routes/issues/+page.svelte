<script lang="ts">
	import IssueCard from '$lib/components/IssueCard.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	function toggleClosed() {
		const url = new URL(window.location.href);
		if (data.showClosed) {
			url.searchParams.delete('closed');
		} else {
			url.searchParams.set('closed', 'true');
		}
		window.location.href = url.toString();
	}
</script>

<div class="max-w-2xl mx-auto p-4">
	<div class="flex items-center justify-between mb-4">
		<h1 class="text-heading-sm font-bold">Ärenden</h1>
		<a href="/issues/new" class="text-sm text-blue-700 hover:underline">Rapportera ett problem →</a>
	</div>

	{#if data.myIssues.length > 0}
		<section class="mb-6">
			<h2 class="text-sm font-medium text-neutral-500 mb-2">Mina ärenden</h2>
			<div class="space-y-2">
				{#each data.myIssues as issue}
					<IssueCard {issue} />
				{/each}
			</div>
		</section>
	{/if}

	{#if data.isManager && data.otherIssues.length > 0}
		<section class="mb-6">
			<h2 class="text-sm font-medium text-neutral-500 mb-2">Övriga ärenden</h2>
			<div class="space-y-2">
				{#each data.otherIssues as issue}
					<IssueCard {issue} />
				{/each}
			</div>
		</section>
	{/if}

	{#if data.myIssues.length === 0 && (!data.isManager || data.otherIssues.length === 0)}
		<p class="text-sm text-neutral-500">Inga ärenden att visa.</p>
	{/if}

	<button
		onclick={toggleClosed}
		class="text-sm text-neutral-500 hover:text-neutral-800 flex items-center gap-1 mt-2"
	>
		<span class="text-xs">{data.showClosed ? '▲' : '▼'}</span>
		{data.showClosed ? 'Dölj avslutade' : 'Visa avslutade'}
	</button>
</div>
