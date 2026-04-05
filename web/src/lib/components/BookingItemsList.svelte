<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';

	interface Props {
		items: BookingItem[];
		editable?: boolean;
		onRemove?: (itemId: string) => void;
	}

	let { items, editable = false, onRemove }: Props = $props();

	const api = createApiClient();
	let latestComments = $state<Map<string, string>>(new Map());

	$effect(() => {
		const needsFetch = items.filter(i => i.article_status !== 'ok' && !latestComments.has(i.article_id));
		if (needsFetch.length === 0) return;
		Promise.all(needsFetch.map(async (i) => {
			try {
				const { events } = await api.listArticleEvents(i.article_id);
				const issueEvent = events.find(e => e.event_type === 'issue_reported' || e.event_type === 'status_change');
				return [i.article_id, issueEvent?.description ?? ''] as const;
			} catch {
				return [i.article_id, ''] as const;
			}
		})).then(results => {
			const next = new Map(latestComments);
			for (const [id, comment] of results) {
				if (comment) next.set(id, comment);
			}
			latestComments = next;
		});
	});

	interface ItemGroup {
		commercialName: string;
		approvalLevel: string;
		items: BookingItem[];
	}

	let groups = $derived.by(() => {
		const map = new Map<string, ItemGroup>();
		for (const item of items) {
			const existing = map.get(item.commercial_name);
			if (existing) {
				existing.items.push(item);
				if (item.approval_level === 'high') existing.approvalLevel = 'high';
				else if (item.approval_level === 'low' && existing.approvalLevel !== 'high') existing.approvalLevel = 'low';
			} else {
				map.set(item.commercial_name, {
					commercialName: item.commercial_name,
					approvalLevel: item.approval_level,
					items: [item]
				});
			}
		}
		return [...map.values()];
	});
</script>

{#if items.length === 0}
	<p class="text-neutral-500">Inga artiklar.</p>
{:else}
	<div class="space-y-2">
		{#each groups as group}
			<div class="border rounded">
				<div class="px-4 py-2 font-medium bg-neutral-50 border-b">
					{group.commercialName} × {group.items.length}
					{#if group.approvalLevel === 'low'}
						<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
					{:else if group.approvalLevel === 'high'}
						<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">Kräver särskilt godkännande</span>
					{/if}
				</div>
				<table class="w-full text-sm">
					<tbody>
						{#each group.items as item}
							<tr class="border-t first:border-t-0">
								<td class="px-4 py-2">
									{item.common_name}
									{#if item.article_status === 'reported_usable'}
										<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Felrapporterad</span>
									{:else if item.article_status === 'incoming'}
										<span class="text-xs bg-blue-50 text-blue-700 border border-blue-200 px-1.5 py-0.5 rounded ml-1">Inkommande{#if item.article_expected_available_date} — {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
									{:else if item.article_status === 'under_repair'}
										<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded ml-1">Under reparation{#if item.article_expected_available_date} — klar {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
									{/if}
									{#if latestComments.has(item.article_id)}
										<p class="text-xs text-neutral-500 italic mt-0.5">“{latestComments.get(item.article_id)}”</p>
									{/if}
								</td>
								<td class="px-4 py-2 text-neutral-600">{item.location_name}</td>
								<td class="px-4 py-2 text-neutral-600">{item.place || ''}</td>
								{#if item.return_status && item.return_status !== 'returned_ok' && item.return_status !== 'pending'}
									<td class="px-4 py-2">
										<span class="text-xs px-1.5 py-0.5 rounded bg-red-100 text-red-700"
										>{{returned_ok: 'OK', delayed: 'Försenad', reported_usable: 'Problem — användbar', reported_unusable: 'Problem — ej användbar', lost: 'Saknas'}[item.return_status] ?? item.return_status}</span>
									</td>
								{:else if !item.pickup_status || item.pickup_status === 'lost'}
									<td class="px-4 py-2"><span class="text-xs text-neutral-400">Ej hämtad</span></td>
								{:else}
									<td></td>
								{/if}
								{#if editable && onRemove}
									<td class="px-4 py-2 text-right">
										<button onclick={() => onRemove(item.id)} class="text-red-600 text-xs hover:underline">Ta bort</button>
									</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/each}
	</div>
{/if}
