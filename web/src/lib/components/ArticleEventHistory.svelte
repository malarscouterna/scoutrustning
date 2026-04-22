<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { createApiClient, type ArticleEvent } from '$lib/api/client';
	import * as m from '$lib/paraglide/messages.js';
	import { msg } from '$lib/msg';
	import { articleEventTypeColors } from '$lib/styles';

	interface Props {
		articleId: string;
	}

	let { articleId }: Props = $props();
	const api = createApiClient();

	const DEFAULT_LIMIT = 6;

	let events = $state<ArticleEvent[]>([]);
	let hasMore = $state(false);
	let loading = $state(true);
	let showingAll = $state(false);
	let containerEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;

	function formatMeta(event: ArticleEvent): string {
		const meta = event.metadata ?? {};
		const parts: string[] = [];
		if (meta.reason === 'lost' || meta.reason === 'missing_at_pickup') parts.push(m.article_status_lost());
		if (meta.new_status && meta.old_status) {
			parts.push(`${msg(`article_status_${meta.old_status}`) ?? meta.old_status} → ${msg(`article_status_${meta.new_status}`) ?? meta.new_status}`);
		} else if (meta.new_status && event.event_type !== 'issue_reported') {
			parts.push(`→ ${msg(`article_status_${meta.new_status}`) ?? meta.new_status}`);
		}
		return parts.join(' · ');
	}

	function hasImages(): boolean {
		return events.some(e => {
			const ids = e.metadata?.image_ids;
			return Array.isArray(ids) && ids.length > 0;
		});
	}

	async function loadEvents(limit?: number) {
		loading = true;
		try {
			const result = await api.listArticleEvents(articleId, limit);
			events = result.events;
			hasMore = result.has_more;
			showingAll = !limit;
		} catch {
			// ignore
		} finally {
			loading = false;
		}
	}

	async function showAll() {
		await loadEvents();
	}

	$effect(() => {
		loadEvents(DEFAULT_LIMIT);
	});

	$effect(() => {
		if (!loading && hasImages() && containerEl && browser && !lightbox) {
			initLightbox();
		}
	});

	onMount(() => {
		return () => { lightbox?.destroy(); lightbox = null; };
	});

	async function initLightbox() {
		const { default: PhotoSwipeLightbox } = await import('photoswipe/lightbox');
		await import('photoswipe/style.css');
		const lb = new PhotoSwipeLightbox({
			gallery: containerEl!,
			children: 'a.pswp-event-img',
			pswpModule: () => import('photoswipe'),
		});
		lb.init();
		lightbox = lb;
	}
</script>

{#if loading}
	<p class="text-xs text-neutral-400 py-1">{m.article_history_loading()}</p>
{:else if events.length === 0}
	<p class="text-xs text-neutral-400 py-1">{m.article_history_empty()}</p>
{:else}
	<div bind:this={containerEl} class="space-y-1.5 mt-1">
		{#each events as event}
			{@const eventImageIds = event.metadata?.image_ids as string[] | undefined}
			<div class="text-xs">
				<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class="text-neutral-400 shrink-0">{new Date(event.created_at).toLocaleDateString('sv')}</span>
					<span class="font-medium {articleEventTypeColors[event.event_type] ?? 'text-neutral-700'}">{msg(`event_type_${event.event_type}`) ?? event.event_type}</span>
					{#if formatMeta(event)}<span class="text-neutral-500">{formatMeta(event)}</span>{/if}
					<span class="text-neutral-400 shrink-0">{event.actor_name}</span>
				</div>
				{#if event.description || eventImageIds?.length}
					<div class="flex flex-wrap items-start gap-2 mt-0.5 pl-0.5">
						{#if eventImageIds?.length}
							{#each eventImageIds as imgId}
								<a
									href="/api/v0/images/{imgId}.webp"
									data-pswp-width="1920"
									data-pswp-height="1440"
									class="pswp-event-img block cursor-zoom-in shrink-0"
								>
									<img src="/api/v0/images/{imgId}_thumb.webp" alt="" class="h-40 rounded object-contain" />
								</a>
							{/each}
						{/if}
						{#if event.description}
							<p class="text-neutral-600 min-w-[10rem] flex-1">{event.description}</p>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
	{#if hasMore && !showingAll}
		<button
			class="text-xs text-blue-600 hover:text-blue-800 mt-2 cursor-pointer"
			onclick={showAll}
		>
			{m.article_history_show_all()}
		</button>
	{/if}
{/if}
