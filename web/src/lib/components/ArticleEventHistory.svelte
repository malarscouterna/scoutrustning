<script lang="ts">
	import { createApiClient, type ArticleEvent } from '$lib/api/client';

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

	const typeLabels: Record<string, string> = {
		issue_reported: 'Problem rapporterat',
		issue_resolved: 'Problem löst',
		status_change: 'Statusändring',
		returned: 'Återlämnad',
		booked: 'Bokad',
		picked_up: 'Uthämtad',
		note: 'Anteckning'
	};

	const typeColors: Record<string, string> = {
		issue_reported: 'text-orange-700',
		issue_resolved: 'text-green-700',
		status_change: 'text-blue-700',
		returned: 'text-green-700',
	};

	const statusLabels: Record<string, string> = {
		ok: 'OK',
		reported_usable: 'Felrapporterad — användbar',
		reported_unusable: 'Felrapporterad — ej användbar',
		under_repair: 'Under reparation',
		lost: 'Saknas',
		archived: 'Arkiverad',
	};

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
				<div class="flex items-start gap-2">
					<span class="text-neutral-400 shrink-0">{new Date(event.created_at).toLocaleDateString('sv')}</span>
					<span class="font-medium {typeColors[event.event_type] ?? 'text-neutral-700'}">{typeLabels[event.event_type] ?? event.event_type}</span>
					{#if formatMeta(event)}<span class="text-neutral-500">{formatMeta(event)}</span>{/if}
					<span class="text-neutral-400 ml-auto shrink-0">{event.actor_name}</span>
				</div>
				{#if event.description}
					<p class="text-neutral-600 ml-[4.5rem] mt-0.5">{event.description}</p>
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
