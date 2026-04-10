<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';

	interface Props {
		imageIds: string[];
		alt: string;
		width?: number;
		height?: number;
		thumbClass?: string;
	}

	let { imageIds, alt, width = 1920, height = 1440, thumbClass = 'w-[300px]' }: Props = $props();

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

		lb.on('uiRegister', () => {
			lb.pswp!.ui!.registerElement({
				name: 'download-button',
				order: 8,
				isButton: true,
				tagName: 'a',
				html: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
				onInit: (el) => {
					el.setAttribute('target', '_blank');
					el.setAttribute('rel', 'noopener');
					el.setAttribute('title', 'Ladda ner');
				},
				onClick: (_e, el) => {
					const src = lb.pswp!.currSlide!.data.src || '';
					const id = src.split('/').pop()?.replace('.webp', '') || '';
					(el as HTMLAnchorElement).href = `/api/v0/images/${id}.webp?format=jpeg`;
					(el as HTMLAnchorElement).download = `${id}.jpg`;
				}
			});
		});

		lb.init();
		lightbox = lb;
	}
</script>

<div bind:this={galleryEl} class="flex gap-2 overflow-x-auto pb-1 scroll-smooth snap-x snap-mandatory">
	{#each imageIds as imgId}
		<a
			href="/api/v0/images/{imgId}.webp"
			data-pswp-width={width}
			data-pswp-height={height}
			class="block shrink-0 cursor-zoom-in snap-start {thumbClass}"
		>
			<img {alt} src="/api/v0/images/{imgId}_thumb.webp" class="w-full rounded aspect-[4/3] object-cover" loading="lazy" />
		</a>
	{/each}
</div>
