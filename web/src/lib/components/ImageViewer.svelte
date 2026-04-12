<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { createApiClient } from '$lib/api/client';

	interface ImageMeta {
		id: string;
		file_id: string;
		title: string;
		description: string;
		format: string;
		attribution: string;
		shared: boolean;
		uploaded_by?: string;
	}

	interface Props {
		imageIds: string[];
		alt: string;
		thumbClass?: string;
		commercialName?: string;
		locationId?: string;
		showMeta?: boolean;
		userId?: string;
		isManager?: boolean;
	}

	let { imageIds, alt, thumbClass = 'h-[225px]', commercialName, locationId, showMeta = false, userId = '', isManager = false }: Props = $props();

	const api = createApiClient();
	let metaMap = $state<Map<string, ImageMeta>>(new Map());
	let galleryEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;

	// Edit state
	let editingMeta = $state<ImageMeta | null>(null);
	let editTitle = $state('');
	let editDescription = $state('');
	let editShared = $state(false);
	let editAttribution = $state('');
	let saving = $state(false);
	let editError = $state('');

	// Source image dimensions by format (longest edge = 2560, square = 2048)
	const formatDimensions: Record<string, { w: number; h: number }> = {
		landscape: { w: 2560, h: 1920 },
		portrait:  { w: 1920, h: 2560 },
		square:    { w: 2048, h: 2048 },
	};

	function getDimensions(imgId: string): { w: number; h: number } {
		const meta = metaMap.get(imgId);
		if (meta?.format && formatDimensions[meta.format]) {
			return formatDimensions[meta.format];
		}
		// Fallback: assume landscape
		return { w: 1920, h: 1440 };
	}

	// Always fetch metadata when commercialName/locationId provided (needed for fullscreen captions + dimensions)
	$effect(() => {
		if (commercialName && locationId && imageIds.length > 0) {
			fetchMeta(commercialName, locationId);
		}
	});

	async function fetchMeta(cn: string, lid: string) {
		try {
			const images = await api.listProductImages(cn, lid);
			const map = new Map<string, ImageMeta>();
			for (const img of images) {
				map.set(img.file_id, img as ImageMeta);
			}
			metaMap = map;
		} catch {
			// Metadata is optional
		}
	}

	function canEdit(meta: ImageMeta): boolean {
		if (isManager) return true;
		if (userId && meta.uploaded_by === userId) return true;
		return false;
	}

	function startEdit(meta: ImageMeta) {
		editingMeta = meta;
		editTitle = meta.title;
		editDescription = meta.description;
		editShared = meta.shared;
		editAttribution = meta.attribution;
		editError = '';
	}

	async function saveEdit() {
		if (!editingMeta) return;
		saving = true;
		editError = '';
		try {
			await api.updateProductImage(editingMeta.id, {
				title: editTitle,
				description: editDescription,
				shared: editShared,
				attribution: editAttribution,
			});
			const updated = { ...editingMeta, title: editTitle, description: editDescription, shared: editShared, attribution: editAttribution };
			const next = new Map(metaMap);
			next.set(editingMeta.file_id, updated);
			metaMap = next;
			editingMeta = null;
		} catch (e: any) {
			editError = e.message ?? 'Kunde inte spara';
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		if (!galleryEl || !browser) return;
		init();
		return () => {
			lightbox?.destroy();
			lightbox = null;
		};
	});

	async function init() {
		const { default: PhotoSwipeLightbox } = await import('photoswipe/lightbox');
		await import('photoswipe/style.css');

		const lb = new PhotoSwipeLightbox({
			gallery: galleryEl!,
			children: 'a.pswp-link',
			pswpModule: () => import('photoswipe'),
			padding: { top: 20, bottom: 40, left: 0, right: 0 },
		});

		lb.on('uiRegister', () => {
			lb.pswp?.ui?.registerElement({
				name: 'custom-caption',
				order: 9,
				isButton: false,
				appendTo: 'root',
				onInit: (el: HTMLElement) => {
					el.style.cssText = 'position:absolute;bottom:0;left:0;right:0;padding:12px 16px;background:linear-gradient(transparent,rgba(0,0,0,.6));color:#fff;font-size:14px;pointer-events:none;';

					function updateCaption() {
						const idx = lb.pswp?.currIndex ?? 0;
						const imgId = imageIds[idx];
						const meta = metaMap.get(imgId);
						if (meta) {
							const lines = [meta.title];
							if (meta.description) lines.push(`<span style="opacity:.8;font-size:12px">${meta.description}</span>`);
							if (meta.attribution) lines.push(`<span style="opacity:.6;font-size:11px">${meta.attribution}</span>`);
							el.innerHTML = lines.join('<br>');
						} else {
							el.innerHTML = '';
						}
					}

					lb.pswp?.on('change', updateCaption);
					updateCaption();
				},
			});
		});

		lb.init();
		lightbox = lb;
	}
</script>

{#if editingMeta}
	<div class="border rounded-lg p-4 bg-white space-y-3">
		<div class="flex items-start gap-4">
			<img src="/api/v0/images/{editingMeta.file_id}_thumb.webp" alt={editingMeta.title} class="h-32 rounded object-contain shrink-0" />
			<div class="flex-1 space-y-3 min-w-0">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Titel</span>
					<input type="text" bind:value={editTitle} class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
					<textarea bind:value={editDescription} rows={2} class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Fotograf</span>
					<input type="text" bind:value={editAttribution} class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={editShared} />
					Dela med andra kårer
				</label>
			</div>
		</div>
		{#if editError}
			<p class="text-xs text-red-600">{editError}</p>
		{/if}
		<div class="flex gap-2">
			<button type="button" onclick={saveEdit} disabled={saving} class="text-sm bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50">{saving ? 'Sparar...' : 'Spara'}</button>
			<button type="button" onclick={() => editingMeta = null} class="text-sm text-neutral-500 underline">Avbryt</button>
		</div>
	</div>
{:else}
	<div bind:this={galleryEl} class="flex gap-2 overflow-x-auto pb-1 scroll-smooth snap-x snap-mandatory">
		{#each imageIds as imgId}
			{@const meta = metaMap.get(imgId)}
			{@const dims = getDimensions(imgId)}
			<div class="shrink-0 snap-start">
				<a
					href="/api/v0/images/{imgId}.webp"
					data-pswp-width={dims.w}
					data-pswp-height={dims.h}
					class="pswp-link block cursor-zoom-in {thumbClass}"
				>
					<img {alt} src="/api/v0/images/{imgId}_thumb.webp" class="h-full rounded object-contain" loading="lazy" />
				</a>
				{#if showMeta && meta}
					<div class="flex items-baseline gap-1 mt-0.5">
						{#if meta.attribution}
							<p class="text-[10px] text-neutral-400 line-clamp-1 flex-1">{meta.attribution}</p>
						{:else}
							<span class="flex-1"></span>
						{/if}
						{#if canEdit(meta)}
							<button type="button" onclick={() => startEdit(meta)} class="text-[10px] text-blue-600 hover:text-blue-800 shrink-0">Redigera</button>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
{/if}
