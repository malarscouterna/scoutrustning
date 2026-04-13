<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { createApiClient } from '$lib/api/client';

	interface Props {
		imageIds: string[];
	}

	let { imageIds = $bindable() }: Props = $props();
	const api = createApiClient();

	let uploading = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);
	let galleryEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;

	async function handleFiles(e: Event) {
		const input = e.target as HTMLInputElement;
		const files = input.files;
		if (!files?.length) return;
		uploading = true;
		try {
			for (const file of files) {
				const result = await api.uploadIssueImage(file);
				imageIds = [...imageIds, result.image_id];
			}
		} catch {
			// silently fail — user sees no thumbnail appear
		} finally {
			uploading = false;
			if (input) input.value = '';
		}
	}

	function remove(id: string) {
		imageIds = imageIds.filter((i) => i !== id);
	}

	onMount(() => {
		if (!browser) return;
		initLightbox();
		return () => { lightbox?.destroy(); lightbox = null; };
	});

	async function initLightbox() {
		const { default: PhotoSwipeLightbox } = await import('photoswipe/lightbox');
		await import('photoswipe/style.css');
		const lb = new PhotoSwipeLightbox({
			gallery: galleryEl!,
			children: 'a.pswp-attach-link',
			pswpModule: () => import('photoswipe'),
		});
		lb.init();
		lightbox = lb;
	}
</script>

<div bind:this={galleryEl} class="flex items-center gap-2 flex-wrap">
	{#each imageIds as id}
		<div class="relative">
			<a
				href="/api/v0/images/{id}.webp"
				data-pswp-width="1920"
				data-pswp-height="1440"
				class="pswp-attach-link block cursor-zoom-in"
			>
				<img src="/api/v0/images/{id}_thumb.webp" alt="" class="h-28 rounded object-contain" />
			</a>
			<button
				type="button"
				onclick={() => remove(id)}
				class="absolute -top-1.5 -right-1.5 w-5 h-5 bg-red-600 text-white rounded-full text-xs leading-none flex items-center justify-center cursor-pointer"
			>×</button>
		</div>
	{/each}
	<button
		type="button"
		onclick={() => fileInput?.click()}
		disabled={uploading}
		class="w-8 h-8 flex items-center justify-center rounded border border-neutral-300 text-neutral-500 hover:bg-neutral-100 cursor-pointer disabled:opacity-50"
		title="Bifoga bild"
	>
		<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
			<path d="M3 4V1h2v3h3v2H5v3H3V6H0V4h3zm3 6V7h3V4h7l1.83 2H21c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H5c-1.1 0-2-.9-2-2V10h3zm7 9c2.76 0 5-2.24 5-5s-2.24-5-5-5-5 2.24-5 5 2.24 5 5 5zm-3.2-5c0 1.77 1.43 3.2 3.2 3.2s3.2-1.43 3.2-3.2-1.43-3.2-3.2-3.2-3.2 1.43-3.2 3.2z"/>
		</svg>
	</button>
	<input
		bind:this={fileInput}
		type="file"
		accept="image/*"
		multiple
		class="hidden"
		onchange={handleFiles}
	/>
	{#if uploading}
		<span class="text-xs text-neutral-400">Laddar upp...</span>
	{/if}
</div>
