<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import { msg } from '$lib/msg';

	interface Props {
		items: BookingItem[];
		editable?: boolean;
		conflictingIds?: Set<string>;
		onRemove?: (itemId: string) => void;
		onAddOne?: (commercialName: string, locationName: string) => void;
		onRemoveOne?: (commercialName: string, locationName: string) => void;
	}

	let { items, editable = false, conflictingIds = new Set(), onRemove, onAddOne, onRemoveOne }: Props = $props();

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

	const statusOrder = ['ok', 'reported_usable', 'incoming', 'reported_unusable', 'under_repair', 'lost', 'archived'] as const;

	interface ItemGroup {
		key: string;
		commercialName: string;
		locationName: string;
		categoryName: string;
		approvalLevel: string;
		imageIds: string[];
		locationId: string;
		description: string;
		instructions: string;
		individuallyTracked: boolean;
		items: BookingItem[];
	}

	interface StatusRow {
		status: string;
		count: number;
		hasConflict: boolean;
	}

	interface LocationSection {
		locationName: string;
		groups: ItemGroup[];
	}

	let allGroups = $derived.by(() => {
		const map = new Map<string, ItemGroup>();
		for (const item of items) {
			const key = item.commercial_name + '||' + item.location_name;
			const existing = map.get(key);
			if (existing) {
				existing.items.push(item);
				if (item.approval_level === 'high') existing.approvalLevel = 'high';
				else if (item.approval_level === 'low' && existing.approvalLevel !== 'high') existing.approvalLevel = 'low';
			} else {
				map.set(key, {
					key,
					commercialName: item.commercial_name,
					locationName: item.location_name,
					categoryName: item.category_name,
					approvalLevel: item.approval_level,
					imageIds: item.image_ids ?? [],
					locationId: item.location_id,
					description: item.article_description ?? '',
					instructions: item.article_instructions ?? '',
					individuallyTracked: item.individually_tracked,
					items: [item]
				});
			}
		}
		return [...map.values()].sort((a, b) =>
			a.categoryName.localeCompare(b.categoryName, 'sv') ||
			a.commercialName.localeCompare(b.commercialName, 'sv')
		);
	});

	let locationSections = $derived.by(() => {
		const sectionMap = new Map<string, LocationSection>();
		for (const group of allGroups) {
			const existing = sectionMap.get(group.locationName);
			if (existing) {
				existing.groups.push(group);
			} else {
				sectionMap.set(group.locationName, { locationName: group.locationName, groups: [group] });
			}
		}
		return [...sectionMap.values()];
	});

	function groupByStatus(groupItems: BookingItem[]): StatusRow[] {
		const map = new Map<string, StatusRow>();
		for (const item of groupItems) {
			const existing = map.get(item.article_status);
			if (existing) {
				existing.count++;
				if (conflictingIds.has(item.article_id)) existing.hasConflict = true;
			} else {
				map.set(item.article_status, {
					status: item.article_status,
					count: 1,
					hasConflict: conflictingIds.has(item.article_id)
				});
			}
		}
		return [...map.values()].sort((a, b) =>
			statusOrder.indexOf(a.status as any) - statusOrder.indexOf(b.status as any)
		);
	}

	function toggleGroup(key: string) {
		const next = new Set(expandedGroups);
		if (next.has(key)) next.delete(key); else next.add(key);
		expandedGroups = next;
	}

	function hasExpandableContent(g: ItemGroup): boolean {
		return g.imageIds.length > 0 || !!g.description || !!g.instructions;
	}
</script>

{#if items.length === 0}
	<p class="text-neutral-500">Inga artiklar.</p>
{:else}
	<div class="space-y-4">
		{#each locationSections as section}
			<div>
				<div class="text-xs font-semibold text-neutral-500 uppercase tracking-wide mb-1 px-1">{section.locationName}</div>
				<div class="space-y-2">
					{#each section.groups as group}
						{@const expandable = hasExpandableContent(group)}
						{@const expanded = expandedGroups.has(group.key)}
						{@const groupCount = group.items.length}
						<div class="border rounded">
							<div class="bg-neutral-50 border-b flex items-stretch">
								<button
									type="button"
									onclick={() => expandable && toggleGroup(group.key)}
									class="flex-1 px-4 py-2 font-medium text-left flex items-center gap-2 min-w-0"
									class:cursor-pointer={expandable}
									class:cursor-default={!expandable}
								>
									<span class="flex-1 min-w-0">
										{group.commercialName}
										{#if group.approvalLevel === 'low'}
											<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
										{:else if group.approvalLevel === 'high'}
											<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">Kräver särskilt godkännande</span>
										{/if}
									</span>
									{#if expandable}
										<span class="text-xs text-neutral-400 shrink-0">{expanded ? '▲' : '▼'}</span>
									{/if}
								</button>
								<!-- Compact quantity controls -->
								{#if onAddOne || onRemoveOne}
									<div class="flex items-center gap-0.5 border-l px-2 shrink-0">
										{#if onRemoveOne}
											<button
												type="button"
												onclick={() => onRemoveOne!(group.commercialName, group.locationName)}
												class="w-7 h-7 rounded border text-sm hover:bg-neutral-100 flex items-center justify-center"
												aria-label="Ta bort en"
											>−</button>
										{/if}
										<span class="w-7 text-center text-sm font-medium">{groupCount}</span>
										{#if onAddOne}
											<button
												type="button"
												onclick={() => onAddOne!(group.commercialName, group.locationName)}
												class="w-7 h-7 rounded border text-sm hover:bg-neutral-100 flex items-center justify-center"
												aria-label="Lägg till en till"
											>+</button>
										{/if}
									</div>
								{/if}
							</div>
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
							{#if group.individuallyTracked}
								<!-- Individually tracked: one row per item -->
								<div class="divide-y">
									{#each group.items as item}
										<div class="px-4 py-2" class:bg-orange-50={conflictingIds.has(item.article_id)}>
											{#if conflictingIds.has(item.article_id)}
												<p class="text-xs text-orange-700 font-medium mb-1">Inte tillgänglig för de valda datumen</p>
											{/if}
											<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
												<span class="text-sm">{item.common_name}</span>
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
													>{({'returned_ok': 'OK', 'delayed': 'Försenad', 'reported_usable': 'Problem - användbar', 'reported_unusable': 'Problem - ej användbar', 'missing': 'Saknas'} as Record<string,string>)[item.return_status!] ?? item.return_status}</span>
												{:else if !item.pickup_status}
													<span class="text-xs text-neutral-400">Ej hämtad</span>
												{/if}
												{#if editable && onRemove}
													<button onclick={() => onRemove(item.id)} class="text-red-600 text-xs hover:underline ml-auto">×</button>
												{/if}
											</div>
											{#if latestComments.has(item.article_id)}
												<p class="text-xs text-neutral-500 italic mt-0.5">"{latestComments.get(item.article_id)}"</p>
											{/if}
										</div>
									{/each}
								</div>
							{:else}
								<!-- Quantity tracked: group by status -->
								<div class="px-4 py-2 divide-y">
									{#each groupByStatus(group.items) as row}
										<div class="py-1.5 flex items-center gap-2">
											<span
												class="text-xs px-1.5 py-0.5 rounded
												{row.status === 'ok' ? 'bg-green-100 text-green-800' :
												 row.status === 'reported_usable' ? 'bg-orange-100 text-orange-700' :
												 row.status === 'reported_unusable' ? 'bg-red-100 text-red-700' :
												 'bg-neutral-100 text-neutral-600'}"
											>×{row.count} {msg(`article_status_${row.status}`) ?? row.status}</span>
											{#if row.hasConflict}
												<span class="text-xs text-orange-700">Inte tillgänglig för de valda datumen</span>
											{/if}
										</div>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
{/if}
