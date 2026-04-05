<script lang="ts">
	import type { PageData } from './$types';
	import type { Article } from '$lib/api/client';
	import ReportIssueForm from '$lib/components/ReportIssueForm.svelte';
	import ArticleEventHistory from '$lib/components/ArticleEventHistory.svelte';

	let { data }: { data: PageData } = $props();

	let search = $state(data.filters.search ?? '');
	let selectedCategory = $state(data.filters.category_id ?? '');
	let selectedLocation = $state(data.filters.location_id ?? '');
	let showArchived = $state(data.filters.status?.includes('archived') ?? false);
	let expandedGroups = $state<Set<string>>(new Set());
	let reportingArticleId = $state<string | null>(null);
	let showHistoryFor = $state<string | null>(null);
	let reportedMessage = $state('');

	const statusOrder = ['ok', 'reported_usable', 'incoming', 'reported_unusable', 'under_repair', 'lost', 'archived'] as const;

	const statusLabels: Record<string, string> = {
		ok: 'OK',
		reported_usable: 'Rapporterad — användbar',
		incoming: 'Inkommande',
		reported_unusable: 'Rapporterad — ej användbar',
		under_repair: 'Under reparation',
		lost: 'Saknas',
		archived: 'Arkiverad',
	};

	const usableStatuses = new Set(['ok', 'reported_usable']);

	function sortByStatus<T extends { status: string }>(items: T[]): T[] {
		return [...items].sort((a, b) => statusOrder.indexOf(a.status as any) - statusOrder.indexOf(b.status as any));
	}

	let articles = $state(data.articles);

	interface ArticleGroup {
		key: string;
		commercialName: string;
		categoryName: string;
		locationName: string;
		count: number;
		articles: Article[];
		individuallyTracked: boolean;
	}

	let groups = $derived.by(() => {
		const map = new Map<string, ArticleGroup>();
		for (const a of articles) {
			const key = `${a.commercial_name}||${a.location_name}`;
			const existing = map.get(key);
			if (existing) {
				existing.count++;
				existing.articles.push(a);
			} else {
				map.set(key, {
					key,
					commercialName: a.commercial_name,
					categoryName: a.category_name,
					locationName: a.location_name,
					count: 1,
					articles: [a],
					individuallyTracked: a.individually_tracked
				});
			}
		}
		return [...map.values()].sort((a, b) =>
			a.categoryName.localeCompare(b.categoryName, 'sv') ||
			a.commercialName.localeCompare(b.commercialName, 'sv')
		);
	});

	function toggleGroup(key: string) {
		if (expandedGroups.has(key)) {
			expandedGroups.delete(key);
		} else {
			expandedGroups.add(key);
		}
		expandedGroups = new Set(expandedGroups);
	}

	function applyFilters() {
		const params = new URLSearchParams();
		if (search) params.set('search', search);
		if (selectedCategory) params.set('category', selectedCategory);
		if (selectedLocation) params.set('location', selectedLocation);
		if (showArchived) params.set('status', 'ok,reported_usable,incoming,reported_unusable,under_repair,lost,archived');
		const qs = params.toString();
		window.location.href = `/browse${qs ? '?' + qs : ''}`;
	}

	function clearFilters() {
		search = '';
		selectedCategory = '';
		selectedLocation = '';
		window.location.href = '/browse';
	}

	function handleIssueReported(newStatus: string) {
		if (reportingArticleId) {
			articles = articles.map((a) => a.id === reportingArticleId ? { ...a, status: newStatus } : a);
		}
		reportingArticleId = null;
		reportedMessage = 'Problem rapporterat!';
		setTimeout(() => reportedMessage = '', 4000);
	}
</script>

<div class="max-w-4xl mx-auto p-4">
	<h1 class="text-heading-sm font-bold mb-4">Utrustning</h1>

	<div class="flex flex-wrap gap-2 mb-4">
		<input
			type="search"
			placeholder="Sök..."
			bind:value={search}
			onkeydown={(e) => e.key === 'Enter' && applyFilters()}
			class="border rounded px-3 py-2 flex-1 min-w-48"
		/>
		<select bind:value={selectedCategory} onchange={applyFilters} class="border rounded px-3 py-2">
			<option value="">Alla kategorier</option>
			{#each data.categories as cat}
				<option value={cat.id}>{cat.name}</option>
			{/each}
		</select>
		<select bind:value={selectedLocation} onchange={applyFilters} class="border rounded px-3 py-2">
			<option value="">Alla platser</option>
			{#each data.locations as loc}
				<option value={loc.id}>{loc.name}</option>
			{/each}
		</select>
		{#if search || selectedCategory || selectedLocation}
			<button onclick={clearFilters} class="text-sm underline px-2">Rensa</button>
		{/if}
	</div>

	<label class="flex items-center gap-2 mb-4 text-sm">
		<input type="checkbox" bind:checked={showArchived} onchange={applyFilters} />
		Visa arkiverade
	</label>

	<p class="text-sm text-neutral-600 mb-4">
		{articles.length} artiklar i {groups.length} grupper
	</p>

	{#if reportedMessage}
		<div class="bg-green-50 border border-green-200 rounded p-3 mb-4 text-green-800 text-sm">{reportedMessage}</div>
	{/if}

	<div class="space-y-1">
		{#each groups as group (group.key)}
			{@const expanded = expandedGroups.has(group.key)}
			{@const usableCount = group.articles.filter(a => usableStatuses.has(a.status)).length}
			<div class="border rounded">
				<button
					onclick={() => toggleGroup(group.key)}
					class="w-full flex items-center justify-between px-4 py-3 hover:bg-neutral-50 text-left"
				>
					<div>
						<span class="font-medium">{group.commercialName}</span>
						<span class="text-sm text-neutral-500 ml-2">{group.categoryName}</span>
					</div>
					<div class="flex items-center gap-3 text-sm text-neutral-600">
						<span>{group.locationName}</span>
						{#if group.individuallyTracked}
							<span class="bg-blue-600 text-white px-2 py-0.5 rounded">{usableCount}/{group.count} st</span>
						{:else}
							<span class="bg-blue-100 text-blue-800 px-2 py-0.5 rounded">×{usableCount}/{group.count}</span>
						{/if}
						<span class="text-xs">{expanded ? '▲' : '▼'}</span>
					</div>
				</button>
				{#if expanded}
					<div class="border-t px-4 py-2 bg-neutral-50">
						{#if group.individuallyTracked}
							<table class="w-full text-sm">
								<thead>
									<tr class="text-left text-neutral-500">
										<th class="py-1">Namn</th>
										<th class="py-1">Plats</th>
										<th class="py-1">Status</th>
										<th class="py-1"></th>
									</tr>
								</thead>
								<tbody>
									{#each sortByStatus(group.articles) as article}
										<tr class="border-t border-neutral-200">
											<td class="py-1">{article.common_name}</td>
											<td class="py-1 text-neutral-600">{article.place || '—'}</td>
											<td class="py-1">
												<span class="inline-block px-2 py-0.5 rounded text-xs
													{article.status === 'ok' ? 'bg-green-100 text-green-800' : article.status.startsWith('reported') ? 'bg-orange-100 text-orange-800' : article.status === 'lost' ? 'bg-challengerpink-100 text-challengerpink-800' : article.status === 'archived' ? 'bg-neutral-100 text-neutral-500' : 'bg-neutral-100'}"
												>
													{statusLabels[article.status] ?? article.status}
												</span>
											</td>
											<td class="py-1 text-right">
												<button onclick={() => reportingArticleId = reportingArticleId === article.id ? null : article.id} class="text-xs text-blue-700 underline">Rapportera</button>
												<button onclick={() => showHistoryFor = showHistoryFor === article.id ? null : article.id} class="text-xs text-neutral-500 underline ml-2">Historik</button>
											</td>
										</tr>
										{#if reportingArticleId === article.id}
											<tr><td colspan="4">
												<ReportIssueForm articleId={article.id} articleName={article.common_name} onReported={handleIssueReported} onCancel={() => reportingArticleId = null} />
											</td></tr>
										{/if}
										{#if showHistoryFor === article.id}
											<tr><td colspan="4" class="py-2">
												<ArticleEventHistory articleId={article.id} />
											</td></tr>
										{/if}
									{/each}
								</tbody>
							</table>
						{:else}
							{@const statusCounts = group.articles.reduce((acc, a) => { acc[a.status] = (acc[a.status] || 0) + 1; return acc; }, {} as Record<string, number>)}
							<div class="flex flex-wrap gap-2 py-1 text-sm">
								{#each statusOrder.filter(s => statusCounts[s]) as status}
									{@const count = statusCounts[status]}
									<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs
										{status === 'ok' ? 'bg-green-100 text-green-800' : status.startsWith('reported') ? 'bg-orange-100 text-orange-800' : status === 'lost' ? 'bg-challengerpink-100 text-challengerpink-800' : status === 'archived' ? 'bg-neutral-100 text-neutral-500' : 'bg-neutral-100'}"
									>
										{count} {statusLabels[status] ?? status}
									</span>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>
