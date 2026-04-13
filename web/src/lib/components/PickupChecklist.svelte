<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import ImageAttachInput from '$lib/components/ImageAttachInput.svelte';

	interface Props {
		bookingId: string;
		items: BookingItem[];
		startDate: string;
		endDate: string;
		onUpdate: (items: BookingItem[]) => void;
	}

	let { bookingId, items, startDate, endDate, onUpdate }: Props = $props();

	const api = createApiClient();

	let error = $state('');
	let swappingItemId = $state<string | null>(null);
	let reportingItemId = $state<string | null>(null);
	let reportingGroupKey = $state<string | null>(null);
	let reportStatus = $state('');
	let reportComment = $state('');
	let reportImageIds = $state<string[]>([]);
	let swapCandidates = $state<{ id: string; common_name: string; location_name: string; place: string; status: string; expected_available_date: string | null }[]>([]);
	let selectedSwapArticle = $state('');
	let loading = $state(false);
	let expandedGroups = $state<Set<string>>(new Set());

	const pickupLabels: Record<string, string> = {
		picked_up: 'Hämtad',
		lost: 'Saknas',
		swapped: 'Bytt'
	};

	interface QuantityGroup {
		commercialName: string;
		locationName: string;
		place: string;
		items: BookingItem[];
	}

	interface TrackedGroup {
		commercialName: string;
		imageIds: string[];
		locationId: string;
		description: string;
		instructions: string;
		items: BookingItem[];
	}

	let trackedItems = $derived(items.filter((i) => i.individually_tracked));
	let quantityGroups = $derived.by(() => {
		const map = new Map<string, QuantityGroup>();
		for (const item of items) {
			if (item.individually_tracked) continue;
			const key = `${item.commercial_name}|${item.location_name}`;
			const existing = map.get(key);
			if (existing) {
				existing.items.push(item);
			} else {
				map.set(key, {
					commercialName: item.commercial_name,
					locationName: item.location_name,
					place: item.place,
					items: [item]
				});
			}
		}
		return [...map.values()];
	});

	let trackedGroups = $derived.by(() => {
		const map = new Map<string, TrackedGroup>();
		for (const item of trackedItems) {
			const existing = map.get(item.commercial_name);
			if (existing) {
				existing.items.push(item);
			} else {
				map.set(item.commercial_name, {
					commercialName: item.commercial_name,
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

	function toggleExpand(key: string) {
		const next = new Set(expandedGroups);
		if (next.has(key)) next.delete(key); else next.add(key);
		expandedGroups = next;
	}

	function hasExpandable(imageIds: string[], desc: string, instr: string): boolean {
		return imageIds.length > 0 || !!desc || !!instr;
	}

	let checkedCount = $derived(items.filter((i) => i.pickup_status !== null).length);

	async function reload() {
		const result = await api.getBooking(bookingId);
		onUpdate(result.items);
	}

	async function markPickup(itemId: string, status: string) {
		error = '';
		try {
			await api.updateItemPickup(bookingId, itemId, status);
			await reload();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function markQuantityGroup(group: QuantityGroup, pickedCount: number) {
		error = '';
		try {
			const extraNeeded = pickedCount - group.items.length;
			if (extraNeeded > 0) {
				await api.addBookingItems(bookingId, group.commercialName, extraNeeded, group.locationName);
				await reload();
				const updatedItems = items.filter(
					(i) => !i.individually_tracked && i.commercial_name === group.commercialName && i.location_name === group.locationName
				);
				for (let i = 0; i < updatedItems.length; i++) {
					const status = i < pickedCount ? 'picked_up' : 'lost';
					await api.updateItemPickup(bookingId, updatedItems[i].id, status);
				}
			} else {
				for (let i = 0; i < group.items.length; i++) {
					const status = i < pickedCount ? 'picked_up' : 'lost';
					await api.updateItemPickup(bookingId, group.items[i].id, status);
				}
			}
			await reload();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function resetQuantityGroup(group: QuantityGroup) {
		error = '';
		try {
			for (const item of group.items) {
				if (item.pickup_status) {
					await api.updateItemPickup(bookingId, item.id, '');
				}
			}
			await reload();
		} catch (e: any) {
			error = e.message;
		}
	}

	function startReport(itemId: string) {
		reportingItemId = itemId;
		reportingGroupKey = null;
		reportStatus = '';
		reportComment = '';
		reportImageIds = [];
	}

	function startGroupReport(group: QuantityGroup) {
		reportingGroupKey = groupKey(group);
		reportingItemId = null;
		reportStatus = '';
		reportComment = '';
		reportImageIds = [];
	}

	function cancelReport() {
		reportingItemId = null;
		reportingGroupKey = null;
		reportStatus = '';
		reportComment = '';
		reportImageIds = [];
	}

	async function confirmReport(itemId: string) {
		if (!reportStatus || !reportComment.trim()) return;
		error = '';
		try {
			const pickupStatus = reportStatus === 'reported_usable' ? 'picked_up' : 'lost';
			await api.updateItemPickup(bookingId, itemId, pickupStatus, reportStatus, reportComment.trim(), reportImageIds.length ? reportImageIds : undefined);
			await reload();
			cancelReport();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function confirmGroupReport(group: QuantityGroup) {
		if (!reportStatus || !reportComment.trim()) return;
		error = '';
		try {
			const target = group.items.find((i) => !i.pickup_status) ?? group.items[0];
			await api.updateArticleStatus(target.article_id, {
				status: reportStatus,
				comment: reportComment.trim(),
				image_ids: reportImageIds.length ? reportImageIds : undefined
			});
			await reload();
			cancelReport();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function startSwap(item: BookingItem) {
		error = '';
		swappingItemId = item.id;
		selectedSwapArticle = '';
		try {
			swapCandidates = await api.listAvailableArticles(startDate, endDate, {
				exclude_booking_id: bookingId,
				commercial_name: item.commercial_name
			});
		} catch (e: any) {
			error = e.message;
			swappingItemId = null;
		}
	}

	function cancelSwap() {
		swappingItemId = null;
		swapCandidates = [];
		selectedSwapArticle = '';
	}

	async function confirmSwap(itemId: string) {
		if (!selectedSwapArticle) return;
		error = '';
		loading = true;
		try {
			await api.swapItem(bookingId, itemId, selectedSwapArticle);
			await reload();
			cancelSwap();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	let quantityInputs = $state<Record<string, number>>({});
	let extraAvailable = $state<Record<string, number>>({});

	function groupKey(group: QuantityGroup): string {
		return `${group.commercialName}|${group.locationName}`;
	}

	function groupPickedCount(group: QuantityGroup): number {
		return group.items.filter((i) => i.pickup_status === 'picked_up' || i.pickup_status === 'swapped').length;
	}

	function groupIsDone(group: QuantityGroup): boolean {
		return group.items.every((i) => i.pickup_status !== null);
	}

	function groupMax(group: QuantityGroup): number {
		const key = groupKey(group);
		return group.items.length + (extraAvailable[key] ?? 0);
	}

	async function loadExtraAvailability() {
		for (const group of quantityGroups) {
			const key = groupKey(group);
			try {
				const available = await api.listAvailableArticles(startDate, endDate, {
					exclude_booking_id: bookingId,
					commercial_name: group.commercialName
				});
				extraAvailable[key] = available.length;
			} catch {
				extraAvailable[key] = 0;
			}
		}
	}

	$effect(() => {
		if (quantityGroups.length > 0) loadExtraAvailability();
	});
</script>

{#snippet infoBlock(imageIds: string[], commercialName: string, locationId: string, description: string, instructions: string)}
	<div class="px-4 py-2 border-t space-y-2 text-xs text-neutral-600">
		{#if imageIds.length > 0}
			<ImageViewer {imageIds} alt={commercialName} {commercialName} {locationId} />
		{/if}
		{#if description}
			<div>
				<span class="font-medium text-neutral-500">Beskrivning:</span>
				<p class="mt-0.5">{description}</p>
			</div>
		{/if}
		{#if instructions}
			<div>
				<span class="font-medium text-neutral-500">Instruktioner:</span>
				<p class="mt-0.5">{instructions}</p>
			</div>
		{/if}
	</div>
{/snippet}

{#if error}
	<div class="bg-red-50 border border-red-200 rounded p-3 mb-3 text-red-800 text-sm">{error}</div>
{/if}

<p class="text-sm text-neutral-500 mb-3">
	Avprickad: {checkedCount} / {items.length}
</p>

<div class="space-y-1">
	<!-- Quantity-tracked groups -->
	{#each quantityGroups as group}
		{@const picked = groupPickedCount(group)}
		{@const done = groupIsDone(group)}
		{@const rep = group.items[0]}
		{@const qImageIds = rep?.image_ids ?? []}
		{@const qKey = groupKey(group)}
		{@const expandable = hasExpandable(qImageIds, rep?.article_description ?? '', rep?.article_instructions ?? '')}
		{@const expanded = expandedGroups.has(qKey)}
		<div class="border rounded" class:bg-green-50={done && picked > 0} class:bg-orange-50={done && picked === 0}>
			<div class="flex items-center gap-3 px-4 py-3">
				<button type="button" onclick={() => expandable && toggleExpand(qKey)} class="flex-1 min-w-0 text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
					<div class="font-medium text-sm">{group.commercialName}{#if expandable}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}</div>
					<div class="text-xs text-neutral-500">{group.locationName}{group.place ? ` · ${group.place}` : ''}</div>
				</button>

				{#if done}
					<span class="text-sm font-medium" class:text-green-800={picked > 0} class:text-orange-800={picked === 0}>
						{picked} / {group.items.length} st hämtade
					</span>
					<button onclick={() => resetQuantityGroup(group)} class="text-xs text-neutral-400 hover:text-neutral-600">Ångra</button>
				{:else}
					{@const key = groupKey(group)}
					{@const max = groupMax(group)}
					{#if reportingGroupKey !== qKey}
					<div class="flex items-center gap-2">
						<span class="text-sm text-neutral-600">Hämta {group.items.length} st</span>
						<input
							type="number"
							min="0"
							{max}
							value={quantityInputs[key] ?? group.items.length}
							oninput={(e) => quantityInputs[key] = parseInt(e.currentTarget.value) || 0}
							class="w-16 text-center border rounded px-2 py-1 text-sm"
						/>
						<button
							onclick={() => markQuantityGroup(group, quantityInputs[key] ?? group.items.length)}
							class="text-xs bg-green-700 text-white px-3 py-1 rounded"
						>Bekräfta</button>
						{#if (extraAvailable[key] ?? 0) > 0}}
							<span class="text-xs text-neutral-400">max {max}</span>
						{/if}
					</div>
					{/if}
				{/if}
			</div>
			{#if expanded}
				{@render infoBlock(qImageIds, group.commercialName, rep?.location_id ?? '', rep?.article_description ?? '', rep?.article_instructions ?? '')}
			{/if}
		</div>
		{#if reportingGroupKey === qKey}
			<div class="border rounded p-3 bg-orange-50 text-sm mt-1">
				<p class="mb-2 font-medium">Rapportera problem — {group.commercialName}:</p>
				<p class="text-xs text-neutral-500 mb-2">Rapporterar på en artikel utan att påverka hämtningen.</p>
				<div class="space-y-2">
					<label class="flex items-center gap-2">
						<input type="radio" name="report-q-{qKey}" value="reported_usable" bind:group={reportStatus} />
						Användbar men felaktig <span class="text-xs text-neutral-500">(hämtas ändå)</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="report-q-{qKey}" value="reported_unusable" bind:group={reportStatus} />
						Ej användbar <span class="text-xs text-neutral-500">(hämtas inte)</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="report-q-{qKey}" value="lost" bind:group={reportStatus} />
						Saknas
					</label>
					<textarea bind:value={reportComment} placeholder="Beskriv problemet..." rows="2" class="block border rounded px-2 py-1 text-sm w-full"></textarea>
					<ImageAttachInput bind:imageIds={reportImageIds} />
					<div class="flex gap-2">
						<button onclick={() => confirmGroupReport(group)} disabled={!reportStatus || !reportComment.trim()} class="text-xs bg-orange-600 text-white px-3 py-1 rounded disabled:opacity-50">Bekräfta</button>
						<button onclick={cancelReport} class="text-xs text-neutral-600 underline">Avbryt</button>
					</div>
				</div>
			</div>
		{/if}
	{/each}

	<!-- Individually tracked items -->
	{#each trackedGroups as tGroup}
		{@const tKey = tGroup.commercialName}
		{@const expandable = hasExpandable(tGroup.imageIds, tGroup.description, tGroup.instructions)}
		{@const expanded = expandedGroups.has(tKey)}
		{#each tGroup.items as item}
		<div class="border rounded" class:bg-green-50={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'} class:bg-challengerpink-50={item.pickup_status === 'lost'}>
			<div class="flex items-center gap-3 px-4 py-3">
				<button type="button" onclick={() => expandable && toggleExpand(tKey)} class="flex-1 min-w-0 text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
					<div class="font-medium text-sm">
						{item.common_name}
						{#if expandable && tGroup.items[0] === item}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}
						{#if item.article_status === 'reported_usable'}
							<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Felrapporterad</span>
						{:else if item.article_status === 'incoming'}
							<span class="text-xs bg-blue-50 text-blue-700 border border-blue-200 px-1.5 py-0.5 rounded ml-1">Inkommande{#if item.article_expected_available_date} — {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
						{:else if item.article_status === 'under_repair'}
							<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded ml-1">Under reparation{#if item.article_expected_available_date} — klar {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
						{/if}
					</div>
					<div class="text-xs text-neutral-500">{item.location_name}{item.place ? ` · ${item.place}` : ''}</div>
				</button>

				<div class="shrink-0">
					{#if item.pickup_status}
						<span class="text-xs px-2 py-0.5 rounded" class:bg-green-100={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'} class:text-green-800={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'} class:bg-challengerpink-100={item.pickup_status === 'lost'} class:text-challengerpink-800={item.pickup_status === 'lost'}>
							{pickupLabels[item.pickup_status] ?? item.pickup_status}
						</span>
						<button onclick={() => markPickup(item.id, '')} class="text-xs text-neutral-400 hover:text-neutral-600 ml-1">Ångra</button>
					{:else if swappingItemId !== item.id && reportingItemId !== item.id}
						<div class="flex gap-1">
							<button onclick={() => markPickup(item.id, 'picked_up')} class="text-xs bg-green-700 text-white px-2 py-1 rounded">Hämtad</button>
							<button onclick={() => startReport(item.id)} class="text-xs bg-orange-600 text-white px-2 py-1 rounded">Rapportera</button>
							<button onclick={() => startSwap(item)} class="text-xs text-blue-700 underline px-1">Byt</button>
						</div>
					{/if}
				</div>
			</div>
			{#if expanded && tGroup.items[0] === item}
				{@render infoBlock(tGroup.imageIds, tGroup.commercialName, tGroup.locationId, tGroup.description, tGroup.instructions)}
			{/if}
		</div>

		{#if reportingItemId === item.id}
			<div class="border rounded p-3 bg-orange-50 text-sm mt-1">
				<p class="mb-2 font-medium">Rapportera {item.common_name}:</p>
				<div class="space-y-2">
					<label class="flex items-center gap-2">
						<input type="radio" name="report-{item.id}" value="reported_usable" bind:group={reportStatus} />
						Användbar men felaktig <span class="text-xs text-neutral-500">(hämtas ändå)</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="report-{item.id}" value="reported_unusable" bind:group={reportStatus} />
						Ej användbar <span class="text-xs text-neutral-500">(hämtas inte)</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="report-{item.id}" value="lost" bind:group={reportStatus} />
						Saknas
					</label>
					<textarea bind:value={reportComment} placeholder="Beskriv problemet..." rows="2" class="block border rounded px-2 py-1 text-sm w-full"></textarea>
					<div class="flex items-center gap-2">
						<ImageAttachInput bind:imageIds={reportImageIds} />
					</div>
					<div class="flex gap-2">
						<button onclick={() => confirmReport(item.id)} disabled={!reportStatus || !reportComment.trim()} class="text-xs bg-orange-600 text-white px-3 py-1 rounded disabled:opacity-50">Bekräfta</button>
						<button onclick={cancelReport} class="text-xs text-neutral-600 underline">Avbryt</button>
					</div>
				</div>
			</div>
		{/if}

		{#if swappingItemId === item.id}
			<div class="border rounded p-3 bg-blue-50 text-sm">
				{#if swapCandidates.length === 0}
					<p class="text-neutral-600">Inga tillgängliga ersättare hittades.</p>
				{:else}
					<p class="mb-2">Välj ersättare för <strong>{item.common_name}</strong>:</p>
					<div class="space-y-1 mb-2">
						{#each swapCandidates as candidate}
							<label class="flex items-center gap-2">
								<input type="radio" name="swap-{item.id}" value={candidate.id} bind:group={selectedSwapArticle} />
								{candidate.common_name}
								{#if candidate.status === 'reported_usable'}
									<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded">Felrapporterad</span>
								{:else if candidate.status === 'incoming'}
									<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">Inkommande{#if candidate.expected_available_date} — {new Date(candidate.expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
								{:else if candidate.status === 'under_repair'}
									<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded">Under reparation{#if candidate.expected_available_date} — klar {new Date(candidate.expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
								{/if}
								<span class="text-xs text-neutral-500">{candidate.location_name}{candidate.place ? ` · ${candidate.place}` : ''}</span>
							</label>
						{/each}
					</div>
				{/if}
				<div class="flex gap-2">
					{#if swapCandidates.length > 0}
						<button onclick={() => confirmSwap(item.id)} disabled={!selectedSwapArticle || loading} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">Byt</button>
					{/if}
					<button onclick={cancelSwap} class="text-xs text-neutral-600 underline">Avbryt</button>
				</div>
			</div>
		{/if}
		{/each}
	{/each}
</div>
