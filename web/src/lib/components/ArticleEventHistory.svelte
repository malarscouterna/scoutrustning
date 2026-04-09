<script lang="ts">
	import { createApiClient, type ArticleEvent } from '$lib/api/client';
	import { statusLabels, eventTypeLabels, eventTypeColors } from '$lib/labels';

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

	function formatMeta(event: ArticleEvent): string {
		const m = event.metadata ?? {};
		const parts: string[] = [];
		if (m.reason === 'lost' || m.reason === 'missing_at_pickup') parts.push('saknas');
		if (m.new_status && m.old_status) {
			parts.push(`${statusLabels[m.old_status] ?? m.old_status} → ${statusLabels[m.new_status] ?? m.new_status}`);
		} else if (m.new_status && event.event_type !== 'issue_reported') {
			parts.push(`→ ${statusLabels[m.new_status] ?? m.new_status}`);
		}
		return parts.join(' · ');
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
</script>

{#if loading}
	<p class="text-xs text-neutral-400 py-1">Laddar historik...</p>
{:else if events.length === 0}
	<p class="text-xs text-neutral-400 py-1">Ingen historik</p>
{:else}
	<div class="space-y-1.5 mt-1">
		{#each events as event}
			<div class="text-xs">
				<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class="text-neutral-400 shrink-0">{new Date(event.created_at).toLocaleDateString('sv')}</span>
					<span class="font-medium {eventTypeColors[event.event_type] ?? 'text-neutral-700'}">{eventTypeLabels[event.event_type] ?? event.event_type}</span>
					{#if formatMeta(event)}<span class="text-neutral-500">{formatMeta(event)}</span>{/if}
					<span class="text-neutral-400 shrink-0">{event.actor_name}</span>
				</div>
				{#if event.description}
					<p class="text-neutral-600 mt-0.5 pl-0.5">{event.description}</p>
				{/if}
			</div>
		{/each}
	</div>
	{#if hasMore && !showingAll}
		<button
			class="text-xs text-blue-600 hover:text-blue-800 mt-2 cursor-pointer"
			onclick={showAll}
		>
			Visa alla händelser
		</button>
	{/if}
{/if}
