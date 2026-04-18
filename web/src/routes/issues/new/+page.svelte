<script lang="ts">
	import { createApiClient, type Article } from '$lib/api/client';
	import { goto } from '$app/navigation';
	import ImageAttachInput from '$lib/components/ImageAttachInput.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	// Article search
	let searchQuery = $state('');
	let searchResults = $state<Article[]>([]);
	let searching = $state(false);
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;
	let selectedArticle = $state<Article | null>(null);
	let showResults = $state(false);

	// Form fields
	let severity = $state(data.prefill.severity);
	let description = $state('');
	let imageIds = $state<string[]>([]);
	let submitting = $state(false);
	let error = $state('');

	// Count for quantity-tracked articles
	let count = $state(1);
	let groupTotal = $state(0);
	let isQuantityTracked = $derived(!selectedArticle?.individually_tracked);

	// Pre-fill article if article_id provided
	$effect(() => {
		if (data.prefill.articleId && !selectedArticle) {
			api.getArticle(data.prefill.articleId).then(a => {
				selectedArticle = a;
				searchQuery = a.commercial_name + (a.common_name ? ' – ' + a.common_name : '');
			}).catch(() => {});
		}
	});

	async function onSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		if (searchQuery.length < 2) { searchResults = []; showResults = false; return; }
		searchTimeout = setTimeout(async () => {
			searching = true;
			try {
				const all = await api.listArticles({ search: searchQuery });
				// Deduplicate: for quantity-tracked groups show only one representative row
				const seen = new Set<string>();
				searchResults = all.filter(a => {
					if (a.individually_tracked) return true;
					const key = a.commercial_name + '|' + a.location_id;
					if (seen.has(key)) return false;
					seen.add(key);
					return true;
				});
				showResults = true;
			} catch { searchResults = []; }
			searching = false;
		}, 200);
	}

	async function selectArticle(article: Article) {
		selectedArticle = article;
		searchQuery = article.commercial_name + (article.common_name ? ' – ' + article.common_name : '');
		showResults = false;
		searchResults = [];
		count = 1;
		groupTotal = 0;
		if (!article.individually_tracked) {
			// Fetch total count for this group to set the max on the count picker
			try {
				const group = await api.listArticles({ search: article.commercial_name, location_id: article.location_id });
				groupTotal = group.filter(a => a.commercial_name === article.commercial_name && a.location_id === article.location_id && a.status !== 'archived').length;
			} catch { /* ignore */ }
		}
	}

	function clearArticle() {
		selectedArticle = null;
		searchQuery = '';
		showResults = false;
	}

	async function submit() {
		if (!selectedArticle) { error = 'Välj utrustning'; return; }
		if (!description.trim()) { error = 'Beskrivning krävs'; return; }
		error = '';
		submitting = true;
		try {
			const issue = await api.createIssue({
				article_id: selectedArticle.id,
				severity,
				description: description.trim(),
				...(data.prefill.bookingId ? { booking_id: data.prefill.bookingId } : {}),
				...(imageIds.length ? { image_ids: imageIds } : {}),
				...(isQuantityTracked && count > 1 ? { count } : {})
			});
			await goto(`/issues/${issue.id}`);
		} catch (e: any) {
			error = e.message ?? 'Något gick fel';
			submitting = false;
		}
	}
</script>

<div class="max-w-lg mx-auto p-4">
	<a href="/issues" class="text-sm text-neutral-500 hover:text-neutral-800 mb-4 inline-block">← Ärenden</a>

	<h1 class="text-heading-sm font-bold mb-6">Rapportera ett problem</h1>

	{#if error}
		<p class="text-red-600 text-sm mb-4">{error}</p>
	{/if}

	<div class="space-y-6">
		<!-- Article search -->
		<div>
			<label for="article-search" class="block text-sm font-medium mb-1">Utrustning</label>
			{#if selectedArticle}
				<div class="flex items-center gap-2 border rounded px-3 py-2 bg-neutral-50">
					<span class="text-sm flex-1">
						<span class="font-medium">{selectedArticle.commercial_name}</span>
						{#if selectedArticle.common_name}
							<span class="text-neutral-500"> – {selectedArticle.common_name}</span>
						{/if}
						<span class="text-neutral-400 text-xs ml-1">({selectedArticle.location_name})</span>
					</span>
					<button onclick={clearArticle} class="text-neutral-400 hover:text-neutral-700 text-lg leading-none" aria-label="Ta bort">×</button>
				</div>
			{:else}
				<div class="relative">
					<input
						id="article-search"
						type="search"
						bind:value={searchQuery}
						oninput={onSearchInput}
						onfocus={() => { if (searchResults.length) showResults = true; }}
						onblur={() => setTimeout(() => { showResults = false; }, 150)}
						placeholder="Sök artikel..."
						class="w-full border rounded px-3 py-2 text-sm"
						autocomplete="off"
					/>
					{#if searching}
						<span class="absolute right-3 top-2.5 text-neutral-400 text-xs">Söker...</span>
					{/if}
					{#if showResults && searchResults.length > 0}
						<ul class="absolute z-10 w-full bg-white border rounded shadow-lg mt-1 max-h-60 overflow-y-auto">
							{#each searchResults as article}
								<li>
									<button
										type="button"
										class="w-full text-left px-3 py-2 text-sm hover:bg-neutral-50"
										onmousedown={() => selectArticle(article)}
									>
										<span class="font-medium">{article.commercial_name}</span>
										{#if article.individually_tracked && article.common_name}
											<span class="text-neutral-500"> – {article.common_name}</span>
										{/if}
										<span class="text-neutral-400 text-xs ml-1">({article.location_name})</span>
									</button>
								</li>
							{/each}
						</ul>
					{:else if showResults && searchQuery.length >= 2 && !searching}
						<div class="absolute z-10 w-full bg-white border rounded shadow-lg mt-1 px-3 py-2 text-sm text-neutral-500">
							Inga träffar
						</div>
					{/if}
				</div>
			{/if}
		</div>

		<!-- Count (quantity-tracked articles only) -->
		{#if selectedArticle && isQuantityTracked}
			<div>
				<label for="count-input" class="block text-sm font-medium mb-1">
					Antal drabbade
					{#if groupTotal > 0}<span class="text-neutral-400 font-normal">(av {groupTotal} st)</span>{/if}
				</label>
				<input
					id="count-input"
					type="number"
					bind:value={count}
					min="1"
					max={groupTotal > 0 ? groupTotal : undefined}
					class="w-24 border rounded px-3 py-2 text-sm"
				/>
			</div>
		{/if}

		<!-- Severity -->
		<fieldset>
			<legend class="block text-sm font-medium mb-2">Allvarlighetsgrad</legend>
			<div class="space-y-2">
				<label class="flex items-center gap-3 cursor-pointer">
					<input type="radio" bind:group={severity} value="usable" class="mt-0.5" />
					<span class="text-sm">
						<span class="font-medium">Användbar</span>
						<span class="text-neutral-500"> – kan fortfarande användas</span>
					</span>
				</label>
				<label class="flex items-center gap-3 cursor-pointer">
					<input type="radio" bind:group={severity} value="unusable" class="mt-0.5" />
					<span class="text-sm">
						<span class="font-medium">Ej användbar</span>
						<span class="text-neutral-500"> – kan inte användas</span>
					</span>
				</label>
				<label class="flex items-center gap-3 cursor-pointer">
					<input type="radio" bind:group={severity} value="missing" class="mt-0.5" />
					<span class="text-sm">
						<span class="font-medium">Saknas</span>
						<span class="text-neutral-500"> – finns inte där den ska finnas</span>
					</span>
				</label>
			</div>
		</fieldset>

		<!-- Description -->
		<div>
			<label for="description" class="block text-sm font-medium mb-1">
				Beskrivning <span class="text-red-500">*</span>
			</label>
			<textarea
				id="description"
				bind:value={description}
				rows="4"
				placeholder="Beskriv problemet..."
				class="w-full border rounded px-3 py-2 text-sm"
			></textarea>
		</div>

		<!-- Images -->
		<div>
			<p class="text-sm font-medium mb-1">Bilder <span class="text-neutral-400 font-normal">(valfritt)</span></p>
			<ImageAttachInput bind:imageIds />
		</div>

		<button
			onclick={submit}
			disabled={submitting || !selectedArticle || !description.trim()}
			class="w-full bg-blue-700 text-white text-sm font-medium py-2.5 rounded disabled:opacity-50"
		>
			{submitting ? 'Skickar...' : 'Skicka rapport'}
		</button>
	</div>
</div>
