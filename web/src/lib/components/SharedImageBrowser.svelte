<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { createApiClient, type SharedImage } from '$lib/api/client';
	import { translateError } from '$lib/errors';

	interface Props {
		commercialName: string;
		locationId: string;
		imageIndex: number;
		existingFileIds?: string[];
		onComplete: (result: { image_ids: string[] }) => void;
		onCancel: () => void;
	}

	let { commercialName, locationId, imageIndex, existingFileIds = [], onComplete, onCancel }: Props = $props();
	const api = createApiClient();

	const formatDimensions: Record<string, { w: number; h: number }> = {
		landscape: { w: 2560, h: 1920 },
		portrait:  { w: 1920, h: 2560 },
		square:    { w: 2048, h: 2048 },
	};

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

	// PhotoSwipe
	let galleryEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;
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

	let existingSet = $derived(new Set(existingFileIds));

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
			error = translateError(e);
		} finally {
			adding = false;
		}
	}

	function openFullscreen(img: SharedImage) {
		if (!browser) return;
		import('photoswipe').then(pswpModule => {
			import('photoswipe/style.css');
			const dims = formatDimensions[img.format] ?? { w: 1920, h: 1440 };
			const pswp = new pswpModule.default({
				dataSource: [{ src: `/api/v0/images/${img.file_id}.webp`, width: dims.w, height: dims.h }],
				index: 0,
				padding: { top: 20, bottom: 40, left: 0, right: 0 },
			});
			pswp.init();
		});
	}

	onMount(() => {
		if (!browser) return;
		initLightbox();
		return () => { lightbox?.destroy(); lightbox = null; };
	});

	async function initLightbox() {
		// Wait for galleryEl to be available
		await new Promise(r => setTimeout(r, 50));
		if (!galleryEl) return;
		const { default: PhotoSwipeLightbox } = await import('photoswipe/lightbox');
		await import('photoswipe/style.css');
		const lb = new PhotoSwipeLightbox({
			gallery: galleryEl,
			children: 'a.pswp-link',
			pswpModule: () => import('photoswipe'),
			padding: { top: 20, bottom: 40, left: 0, right: 0 },
		});
		lb.init();
		lightbox = lb;
	}

	// Re-init lightbox when images change
	$effect(() => {
		if (images.length > 0 && galleryEl && !selected) {
			lightbox?.destroy();
			initLightbox();
		}
	});
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
					<button type="button" onclick={() => openFullscreen(selected!)} class="cursor-zoom-in">
						<img src="/api/v0/images/{selected.file_id}.webp" alt={selected.title} class="max-h-64 rounded object-contain" />
					</button>
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
					<div bind:this={galleryEl} class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
						{#each sortedImages as img}
							{@const dims = formatDimensions[img.format] ?? { w: 1920, h: 1440 }}
							{@const alreadyAdded = existingSet.has(img.file_id)}
							<div class="border rounded overflow-hidden transition-colors" class:hover:border-blue-400={!alreadyAdded} class:hover:shadow={!alreadyAdded} class:opacity-40={alreadyAdded}>
								<div class="bg-neutral-100 flex justify-center p-1">
									<a
										href="/api/v0/images/{img.file_id}.webp"
										data-pswp-width={dims.w}
										data-pswp-height={dims.h}
										class="pswp-link block cursor-zoom-in"
									>
										<img src="/api/v0/images/{img.file_id}_thumb.webp" alt={img.title} class="h-32 sm:h-40 md:h-44 object-contain" loading="lazy" />
									</a>
								</div>
								<div class="p-2 space-y-0.5">
									<div class="flex flex-wrap items-center gap-1">
										<span class="text-xs font-medium flex-1">{img.title}</span>
										{#if isMatch(img)}
											<span class="shrink-0 text-[10px] bg-green-100 text-green-700 px-1 rounded">Match</span>
										{/if}
										{#if alreadyAdded}
											<span class="shrink-0 text-[10px] text-neutral-400 ml-auto">Redan tillagd</span>
										{:else}
											<button
												type="button"
												onclick={() => selectImage(img)}
												class="shrink-0 text-xs text-blue-700 border border-blue-200 bg-blue-50 rounded px-2 py-0.5 hover:bg-blue-100 ml-auto"
											>Välj</button>
										{/if}
									</div>
									{#if img.attribution}
										<p class="text-[10px] text-neutral-400 line-clamp-1">{img.attribution}</p>
									{/if}
									{#if img.description}
										<p class="text-[10px] text-neutral-500 line-clamp-2">{img.description}</p>
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
