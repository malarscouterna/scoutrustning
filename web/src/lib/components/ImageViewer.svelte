<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';

	interface Props {
		imageIds: string[];
		alt: string;
		thumbClass?: string;
	}

	let { imageIds, alt, thumbClass = 'h-[225px]' }: Props = $props();

	let galleryEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;

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
			children: 'a',
			pswpModule: () => import('photoswipe'),
			padding: { top: 20, bottom: 20, left: 0, right: 0 },
		});

		// Auto-detect image dimensions so non-4:3 images aren't distorted
		lb.addFilter('itemData', (itemData: any) => {
			const img = new Image();
			img.src = itemData.src;
			if (img.naturalWidth) {
				itemData.w = img.naturalWidth;
				itemData.h = img.naturalHeight;
			}
			return itemData;
		});

		lb.on('contentLoad', (e: any) => {
			const { content } = e;
			if (content.type === 'image') {
				const img = new Image();
				img.onload = () => {
					content.width = img.naturalWidth;
					content.height = img.naturalHeight;
					content.instance?.updateSize(true);
				};
				img.src = content.data.src;
			}
		});

		lb.init();
		lightbox = lb;
	}
</script>

<div bind:this={galleryEl} class="flex gap-2 overflow-x-auto pb-1 scroll-smooth snap-x snap-mandatory">
	{#each imageIds as imgId}
		<a
			href="/api/v0/images/{imgId}.webp"
			class="block shrink-0 cursor-zoom-in snap-start {thumbClass}"
		>
			<img {alt} src="/api/v0/images/{imgId}_thumb.webp" class="h-full rounded object-contain" loading="lazy" />
		</a>
	{/each}
</div>
