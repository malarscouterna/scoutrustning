<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';

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
	let reportStatus = $state('');
	let reportComment = $state('');
	let swapCandidates = $state<{ id: string; common_name: string; location_name: string; place: string; status: string; expected_available_date: string | null }[]>([]);
	let selectedSwapArticle = $state('');
	let loading = $state(false);

	const pickupLabels: Record<string, string> = {
		picked_up: 'Hämtad',
		lost: 'Saknas',
		swapped: 'Bytt'
	};

	// Split items into individually tracked and quantity groups
	interface QuantityGroup {
		commercialName: string;
		locationName: string;
		place: string;
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
			// If picking more than booked, add extra items first
			const extraNeeded = pickedCount - group.items.length;
			if (extraNeeded > 0) {
				await api.addBookingItems(bookingId, group.commercialName, extraNeeded, group.locationName);
				await reload();
				// After reload, items prop has changed — re-derive the group
				// and mark all items in the updated group
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
		reportStatus = '';
		reportComment = '';
	}

	function cancelReport() {
		reportingItemId = null;
		reportStatus = '';
		reportComment = '';
	}

	async function confirmReport(itemId: string) {
		if (!reportStatus || !reportComment.trim()) return;
		error = '';
		try {
			// reported_unusable and lost → don't pick up (mark as lost in booking)
			// reported_usable → pick up but flag the issue
			const pickupStatus = reportStatus === 'reported_usable' ? 'picked_up' : 'lost';
			await api.updateItemPickup(bookingId, itemId, pickupStatus, reportStatus, reportComment.trim());
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

	// Fetch extra availability for quantity groups on load
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
		<div class="border rounded px-4 py-3" class:bg-green-50={done && picked > 0} class:bg-orange-50={done && picked === 0}>
			<div class="flex items-center gap-3">
				<div class="flex-1 min-w-0">
					<div class="font-medium text-sm">{group.commercialName}</div>
					<div class="text-xs text-neutral-500">{group.locationName}{group.place ? ` · ${group.place}` : ''}</div>
				</div>

				{#if done}
					<span class="text-sm font-medium" class:text-green-800={picked > 0} class:text-orange-800={picked === 0}>
						{picked} / {group.items.length} st hämtade
					</span>
					<button onclick={() => resetQuantityGroup(group)} class="text-xs text-neutral-400 hover:text-neutral-600">Ångra</button>
				{:else}
					{@const key = groupKey(group)}
					{@const max = groupMax(group)}
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
						{#if (extraAvailable[key] ?? 0) > 0}
							<span class="text-xs text-neutral-400">max {max}</span>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	{/each}

	<!-- Individually tracked items -->
	{#each trackedItems as item}
		<div class="border rounded px-4 py-3 flex items-center gap-3" class:bg-green-50={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'} class:bg-challengerpink-50={item.pickup_status === 'lost'}>
			<div class="flex-1 min-w-0">
				<div class="font-medium text-sm">
					{item.common_name}
					{#if item.article_status === 'reported_usable'}
						<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Felrapporterad</span>
					{:else if item.article_status === 'incoming'}
						<span class="text-xs bg-blue-50 text-blue-700 border border-blue-200 px-1.5 py-0.5 rounded ml-1">Inkommande{#if item.article_expected_available_date} — {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
					{:else if item.article_status === 'under_repair'}
						<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded ml-1">Under reparation{#if item.article_expected_available_date} — klar {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
					{/if}
				</div>
				<div class="text-xs text-neutral-500">{item.location_name}{item.place ? ` · ${item.place}` : ''}</div>
			</div>

			{#if item.pickup_status}
				<span class="text-xs px-2 py-0.5 rounded" class:bg-green-100={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'} class:text-green-800={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'} class:bg-challengerpink-100={item.pickup_status === 'lost'} class:text-challengerpink-800={item.pickup_status === 'lost'}>
					{pickupLabels[item.pickup_status] ?? item.pickup_status}
				</span>
				<button onclick={() => markPickup(item.id, '')} class="text-xs text-neutral-400 hover:text-neutral-600">Ångra</button>
			{:else if swappingItemId !== item.id && reportingItemId !== item.id}
				<div class="flex gap-1">
					<button onclick={() => markPickup(item.id, 'picked_up')} class="text-xs bg-green-700 text-white px-2 py-1 rounded">Hämtad</button>
					<button onclick={() => startReport(item.id)} class="text-xs bg-orange-600 text-white px-2 py-1 rounded">Rapportera</button>
					<button onclick={() => startSwap(item)} class="text-xs text-blue-700 underline px-1">Byt</button>
				</div>
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
</div>
