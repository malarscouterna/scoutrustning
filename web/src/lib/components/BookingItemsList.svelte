<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';
	import ImageViewer from '$lib/components/ImageViewer.svelte';

	interface Props {
		items: BookingItem[];
		editable?: boolean;
		onRemove?: (itemId: string) => void;
	}

	let { items, editable = false, onRemove }: Props = $props();

	const api = createApiClient();
	let latestComments = $state<Map<string, string>>(new Map());
	let expandedGroups = $state<Set<string>>(new Set());

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
		imageIds: string[];
		locationId: string;
		description: string;
		instructions: string;
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
					imageIds: item.image_ids ?? [],
					locationId: item.location_id,
					description: item.article_description ?? '',
					instructions: item.article_instructions ?? '',
					items: [item]
				});
			}
		}
		return [...map.values()];
	});

	function toggleGroup(name: string) {
		const next = new Set(expandedGroups);
		if (next.has(name)) next.delete(name); else next.add(name);
		expandedGroups = next;
	}

	function hasExpandableContent(g: ItemGroup): boolean {
		return g.imageIds.length > 0 || !!g.description || !!g.instructions;
	}
</script>

{#if items.length === 0}
	<p class="text-neutral-500">Inga artiklar.</p>
{:else}
	<div class="space-y-2">
		{#each groups as group}
			{@const expandable = hasExpandableContent(group)}
			{@const expanded = expandedGroups.has(group.commercialName)}
			<div class="border rounded">
				<button
					type="button"
					onclick={() => expandable && toggleGroup(group.commercialName)}
					class="w-full px-4 py-2 font-medium bg-neutral-50 border-b text-left flex items-center gap-2"
					class:cursor-pointer={expandable}
					class:cursor-default={!expandable}
				>
					<span class="flex-1">
						{group.commercialName} × {group.items.length}
						{#if group.approvalLevel === 'low'}
							<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
						{:else if group.approvalLevel === 'high'}
							<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">Kräver särskilt godkännande</span>
						{/if}
					</span>
					{#if expandable}
						<span class="text-xs text-neutral-400">{expanded ? '▲' : '▼'}</span>
					{/if}
				</button>
				{#if expanded}
					<div class="px-4 py-2 border-b bg-white space-y-2 text-xs text-neutral-600">
						{#if group.imageIds.length > 0}
							<ImageViewer imageIds={group.imageIds} alt={group.commercialName} commercialName={group.commercialName} locationId={group.locationId} />
						{/if}
						{#if group.description}
							<div>
								<span class="font-medium text-neutral-500">Beskrivning:</span>
								<p class="mt-0.5">{group.description}</p>
							</div>
						{/if}
						{#if group.instructions}
							<div>
								<span class="font-medium text-neutral-500">Instruktioner:</span>
								<p class="mt-0.5">{group.instructions}</p>
							</div>
						{/if}
					</div>
				{/if}
				<div class="divide-y">
					{#each group.items as item}
						<div class="px-4 py-2">
							<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
								<span class="text-sm">{item.common_name}</span>
								<span class="text-sm text-neutral-600">{item.location_name}</span>
								{#if item.place}<span class="text-sm text-neutral-600">{item.place}</span>{/if}
								{#if item.article_status === 'reported_usable'}
									<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded">Felrapporterad</span>
								{:else if item.article_status === 'incoming'}
									<span class="text-xs bg-blue-50 text-blue-700 border border-blue-200 px-1.5 py-0.5 rounded">Inkommande{#if item.article_expected_available_date} — {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
								{:else if item.article_status === 'under_repair'}
									<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded">Under reparation{#if item.article_expected_available_date} — klar {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
								{/if}
								{#if item.return_status && item.return_status !== 'returned_ok' && item.return_status !== 'pending'}
									<span class="text-xs px-1.5 py-0.5 rounded bg-red-100 text-red-700"
									>{{returned_ok: 'OK', delayed: 'Försenad', reported_usable: 'Problem — användbar', reported_unusable: 'Problem — ej användbar', lost: 'Saknas'}[item.return_status] ?? item.return_status}</span>
								{:else if !item.pickup_status || item.pickup_status === 'lost'}
									<span class="text-xs text-neutral-400">Ej hämtad</span>
								{/if}
								{#if editable && onRemove}
									<button onclick={() => onRemove(item.id)} class="text-red-600 text-xs hover:underline ml-auto">Ta bort</button>
								{/if}
							</div>
							{#if latestComments.has(item.article_id)}
								<p class="text-xs text-neutral-500 italic mt-0.5">"{latestComments.get(item.article_id)}"</p>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
{/if}
