<script lang="ts">
	import type { PageData } from './$types';
	import type { Article } from '$lib/api/client';

	let { data }: { data: PageData } = $props();

	let search = $state(data.filters.search ?? '');
	let selectedCategory = $state(data.filters.category_id ?? '');
	let selectedLocation = $state(data.filters.location_id ?? '');
	let expandedGroups = $state<Set<string>>(new Set());

	interface ArticleGroup {
		key: string;
		commercialName: string;
		categoryName: string;
		locationName: string;
		count: number;
		articles: Article[];
	}

	let groups = $derived.by(() => {
		const map = new Map<string, ArticleGroup>();
		for (const a of data.articles) {
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
					articles: [a]
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
		const qs = params.toString();
		window.location.href = `/browse${qs ? '?' + qs : ''}`;
	}

	function clearFilters() {
		search = '';
		selectedCategory = '';
		selectedLocation = '';
		window.location.href = '/browse';
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

	<p class="text-sm text-neutral-600 mb-4">
		{data.articles.length} artiklar i {groups.length} grupper
	</p>

	<div class="space-y-1">
		{#each groups as group (group.key)}
			{@const expanded = expandedGroups.has(group.key)}
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
						<span class="bg-neutral-100 px-2 py-0.5 rounded">{group.count} st</span>
						<span class="text-xs">{expanded ? '▲' : '▼'}</span>
					</div>
				</button>
				{#if expanded}
					<div class="border-t px-4 py-2 bg-neutral-50">
						<table class="w-full text-sm">
							<thead>
								<tr class="text-left text-neutral-500">
									<th class="py-1">Namn</th>
									<th class="py-1">Plats</th>
									<th class="py-1">Status</th>
								</tr>
							</thead>
							<tbody>
								{#each group.articles as article}
									<tr class="border-t border-neutral-200">
										<td class="py-1">{article.common_name}</td>
										<td class="py-1 text-neutral-600">{article.place || '—'}</td>
										<td class="py-1">
											<span class="inline-block px-2 py-0.5 rounded text-xs
												{article.status === 'ok' ? 'bg-green-100 text-green-800' : 'bg-neutral-100'}"
											>
												{article.status}
											</span>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>
