<script lang="ts">
	import { createApiClient, type SharedImage } from '$lib/api/client';

	interface Props {
		commercialName: string;
		locationId: string;
		imageIndex: number;
		onComplete: (result: { image_ids: string[] }) => void;
		onCancel: () => void;
	}

	let { commercialName, locationId, imageIndex, onComplete, onCancel }: Props = $props();
	const api = createApiClient();

	let search = $state('');
	let images = $state<SharedImage[]>([]);
	let loading = $state(true);
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	// Confirm phase
	let selected = $state<SharedImage | null>(null);
	let title = $state('');
	let description = $state('');
	let adding = $state(false);
	let error = $state('');
	let expandedId = $state<string | null>(null);
	// Initial load
	loadImages('');

	function onSearchInput() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => loadImages(search), 300);
	}

	async function loadImages(q: string) {
		loading = true;
		try {
			images = await api.listSharedImages(q || undefined);
		} catch {
			images = [];
		} finally {
			loading = false;
		}
	}

	function isMatch(img: SharedImage): boolean {
		const cn = commercialName.toLowerCase();
		return img.title.toLowerCase().includes(cn) || img.description.toLowerCase().includes(cn);
	}

	let sortedImages = $derived(
		[...images].sort((a, b) => {
			const am = isMatch(a) ? 0 : 1;
			const bm = isMatch(b) ? 0 : 1;
			return am - bm;
		})
	);

	function selectImage(img: SharedImage) {
		selected = img;
		title = `${commercialName} ${imageIndex}`;
		description = img.description;
		error = '';
	}

	async function handleAdd() {
		if (!selected) return;
		adding = true;
		error = '';
		try {
			const result = await api.addFromShared(selected.id, commercialName, locationId, title, description);
			onComplete(result);
		} catch (e: any) {
			error = e.message ?? 'Kunde inte lägga till bilden';
		} finally {
			adding = false;
		}
	}
</script>

<div class="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
	<div class="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] flex flex-col">
		{#if selected}
			<!-- Confirm phase -->
			<div class="p-4 border-b flex items-center justify-between">
				<h3 class="font-medium text-sm">Lägg till delad bild</h3>
				<button type="button" onclick={() => selected = null} class="text-sm text-neutral-500 hover:text-neutral-700">← Tillbaka</button>
			</div>
			<div class="p-4 space-y-4 overflow-y-auto">
				<div class="bg-neutral-100 rounded p-2 flex justify-center">
					<img src="/api/v0/images/{selected.file_id}_thumb.webp" alt={selected.title} class="max-h-48 rounded object-contain" />
				</div>

				{#if selected.attribution}
					<p class="text-xs text-neutral-400">{selected.attribution}</p>
				{/if}

				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Titel</span>
					<input type="text" bind:value={title} class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>

				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
					<textarea bind:value={description} rows={2} class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
				</label>

				{#if error}
					<p class="text-xs text-red-600">{error}</p>
				{/if}

				<div class="flex gap-2 pt-2">
					<button
						type="button"
						onclick={handleAdd}
						disabled={adding}
						class="bg-blue-700 text-white px-4 py-2 rounded text-sm disabled:opacity-50"
					>{adding ? 'Lägger till...' : 'Lägg till'}</button>
					<button type="button" onclick={() => selected = null} class="text-sm text-neutral-500 underline">Tillbaka</button>
				</div>
			</div>
		{:else}
			<!-- Browse phase -->
			<div class="p-4 border-b space-y-2">
				<div class="flex items-center justify-between">
					<h3 class="font-medium text-sm">Bläddra bland bilder</h3>
					<button type="button" onclick={onCancel} class="text-sm text-neutral-500 hover:text-neutral-700">Stäng</button>
				</div>
				<label class="block">
					<span class="sr-only">Sök bilder</span>
					<input type="search" bind:value={search} oninput={onSearchInput} placeholder="Sök på titel eller beskrivning..." class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
			</div>

			<div class="flex-1 overflow-y-auto p-4">
				{#if loading}
					<p class="text-sm text-neutral-400 text-center py-8">Laddar...</p>
				{:else if sortedImages.length === 0}
					<p class="text-sm text-neutral-400 text-center py-8">Inga bilder hittades</p>
				{:else}
					<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
						{#each sortedImages as img}
							{@const expanded = expandedId === img.id}
							<div class="border rounded overflow-hidden hover:border-blue-400 hover:shadow transition-colors">
								<button
									type="button"
									onclick={() => selectImage(img)}
									class="w-full text-left"
								>
									<div class="bg-neutral-100 flex justify-center p-1">
										<img src="/api/v0/images/{img.file_id}_thumb.webp" alt={img.title} class="h-32 sm:h-40 md:h-44 object-contain" loading="lazy" />
									</div>
								</button>
								<div class="p-2 space-y-0.5">
									<div class="flex items-start gap-1">
										<span class="text-xs font-medium line-clamp-1 flex-1">{img.title}</span>
										{#if isMatch(img)}
											<span class="shrink-0 text-[10px] bg-green-100 text-green-700 px-1 rounded">Match</span>
										{/if}
									</div>
									{#if img.attribution}
										<p class="text-[10px] text-neutral-400 line-clamp-1">{img.attribution}</p>
									{/if}
									{#if img.description}
										<button
											type="button"
											onclick={() => expandedId = expanded ? null : img.id}
											class="text-[10px] text-blue-600 hover:text-blue-800"
										>{expanded ? 'Dölj ▲' : 'Detaljer ▼'}</button>
									{/if}
									{#if expanded}
										<p class="text-[11px] text-neutral-500">{img.description}</p>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	</div>
</div>
