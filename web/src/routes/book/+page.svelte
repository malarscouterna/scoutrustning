<script lang="ts">
	import { createApiClient, type AvailabilityGroup, type BookingItem } from '$lib/api/client';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const api = createApiClient({ persona: 'leader-yggdrasil' });

	let startDate = $state('');
	let endDate = $state('');
	let notes = $state('');
	let selectedUnit = $state('');
	let selectedCategory = $state('');
	let selectedLocation = $state('');
	let searchQuery = $state('');
	let showRequiresApproval = $state(false);
	let availability = $state<AvailabilityGroup[]>([]);
	// Stable list that keeps groups visible even when they hit 0 available
	let stableAvailability = $state<AvailabilityGroup[]>([]);
	let bookingId = $state<string | null>(null);
	let cartItems = $state<BookingItem[]>([]);
	let quantities = $state<Record<string, number>>({});
	let error = $state('');
	let submitted = $state(false);
	let loading = $state(false);

	let filteredAvailability = $derived(
		searchQuery
			? stableAvailability.filter(g => g.commercial_name.toLowerCase().includes(searchQuery.toLowerCase()))
			: stableAvailability
	);
	// Count booked items per commercial_name+location from cart
	let bookedCounts = $derived.by(() => {
		const counts: Record<string, number> = {};
		for (const item of cartItems) {
			const key = item.commercial_name + '||' + item.location_name;
			counts[key] = (counts[key] ?? 0) + 1;
		}
		return counts;
	});

	async function checkAvailability() {
		if (!startDate || !endDate) return;
		error = '';
		loading = true;
		try {
			availability = await api.checkAvailability(startDate, endDate, {
				category_id: selectedCategory || undefined,
				location_id: selectedLocation || undefined,
				bookable_only: !showRequiresApproval
			});
			stableAvailability = [...availability];

			if (!bookingId) {
				const booking = await api.createBooking({
					start_date: startDate,
					end_date: endDate,
					notes,
					used_by_unit_id: selectedUnit || undefined
				});
				bookingId = booking.id;
			}
		} catch (e: any) {
			error = e.message;
		}
		loading = false;
	}

	async function refreshAvailability() {
		if (!startDate || !endDate) return;
		try {
			availability = await api.checkAvailability(startDate, endDate, {
				category_id: selectedCategory || undefined,
				location_id: selectedLocation || undefined,
				bookable_only: !showRequiresApproval
			});
			// Merge: update counts for existing groups, keep groups that hit 0
			const freshMap = new Map<string, AvailabilityGroup>();
			for (const g of availability) {
				freshMap.set(g.commercial_name + '||' + g.location_name, g);
			}
			stableAvailability = stableAvailability.map(g => {
				const key = g.commercial_name + '||' + g.location_name;
				return freshMap.get(key) ?? { ...g, available_count: 0 };
			});
		} catch (_) {}
	}

	async function addToCart(commercialName: string, locationName: string) {
		if (!bookingId) return;
		const qty = quantities[commercialName + '||' + locationName] || 1;
		error = '';
		try {
			await api.addBookingItems(bookingId, commercialName, qty, locationName);
			const result = await api.getBooking(bookingId);
			cartItems = result.items;
			quantities[commercialName + '||' + locationName] = 1;
			await refreshAvailability();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function removeFromCart(itemId: string) {
		if (!bookingId) return;
		error = '';
		try {
			await api.removeBookingItem(bookingId, itemId);
			const result = await api.getBooking(bookingId);
			cartItems = result.items;
			await refreshAvailability();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function submitBooking() {
		if (!bookingId) return;
		error = '';
		try {
			await api.submitBooking(bookingId);
			submitted = true;
		} catch (e: any) {
			error = e.message;
		}
	}

	async function cancelDraft() {
		if (!bookingId) return;
		try {
			await api.cancelBooking(bookingId);
			// Reset everything
			bookingId = null;
			cartItems = [];
			availability = [];
			quantities = {};
			error = '';
		} catch (e: any) {
			error = e.message;
		}
	}

	function onFilterChange() {
		if (startDate && endDate) {
			checkAvailability();
		}
	}

	// Group cart items by commercial_name
	let cartGroups = $derived.by(() => {
		const map = new Map<string, { commercialName: string; items: BookingItem[] }>();
		for (const item of cartItems) {
			const existing = map.get(item.commercial_name);
			if (existing) {
				existing.items.push(item);
			} else {
				map.set(item.commercial_name, { commercialName: item.commercial_name, items: [item] });
			}
		}
		return [...map.values()];
	});
</script>

<div class="max-w-4xl mx-auto p-4">
	<h1 class="text-heading-sm font-bold mb-4">Boka utrustning</h1>

	{#if submitted}
		<div class="bg-green-50 border border-green-200 rounded p-4">
			<p class="font-medium text-green-800">Bokningen är inskickad!</p>
			<a href="/bookings/{bookingId}" class="underline text-green-700">Visa bokning →</a>
		</div>
	{:else}
		<div class="flex flex-wrap gap-3 mb-4">
			<label class="flex flex-col gap-1">
				<span class="text-sm">Startdatum</span>
				<input type="date" bind:value={startDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">Slutdatum</span>
				<input type="date" bind:value={endDate} class="border rounded px-3 py-2" />
			</label>
			<div class="flex items-end">
				<button
					onclick={checkAvailability}
					disabled={!startDate || !endDate || loading}
					class="bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50"
				>
					{loading ? '...' : 'Visa tillgänglighet'}
				</button>
			</div>
		</div>

		<label class="flex flex-col gap-1 mb-4">
			<span class="text-sm">Anteckningar</span>
			<input type="text" bind:value={notes} placeholder="T.ex. Hajk med Yggdrasil" class="border rounded px-3 py-2" />
		</label>

		<div class="flex flex-wrap gap-3 mb-4">
			<label class="flex flex-col gap-1">
				<span class="text-sm">Bokas för</span>
				<select bind:value={selectedUnit} class="border rounded px-3 py-2">
					<option value="">Personlig bokning</option>
					{#each data.units as unit}
						<option value={unit.id}>{unit.name}</option>
					{/each}
				</select>
			</label>
		</div>

		{#if error}
			<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{error}</div>
		{/if}

		{#if stableAvailability.length > 0}
			<div class="flex flex-wrap gap-2 mb-3">
				<input
					type="search"
					placeholder="Sök..."
					bind:value={searchQuery}
					class="border rounded px-3 py-2 text-sm flex-1 min-w-48"
				/>
				<select bind:value={selectedCategory} onchange={onFilterChange} class="border rounded px-3 py-2 text-sm">
					<option value="">Alla kategorier</option>
					{#each data.categories as cat}
						<option value={cat.id}>{cat.name}</option>
					{/each}
				</select>
				<select bind:value={selectedLocation} onchange={onFilterChange} class="border rounded px-3 py-2 text-sm">
					<option value="">Alla platser</option>
					{#each data.locations as loc}
						<option value={loc.id}>{loc.name}</option>
					{/each}
				</select>
				<label class="flex items-center gap-1.5 text-sm">
					<input type="checkbox" bind:checked={showRequiresApproval} onchange={onFilterChange} />
					Visa även låst utrustning
				</label>
			</div>

			<h2 class="font-medium mb-2">Tillgänglig utrustning</h2>
			<div class="space-y-1 mb-6">
				{#each filteredAvailability as group}
					{@const key = group.commercial_name + '||' + group.location_name}
					{@const booked = bookedCounts[key] ?? 0}
					<div class="flex items-center justify-between border rounded px-4 py-2">
						<div>
							<span class="font-medium">{group.commercial_name}</span>
							<span class="text-xs text-neutral-500 ml-2">{group.category_name} · {group.location_name}</span>
							{#if group.requires_approval}
								<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
							{/if}
						</div>
						<div class="flex items-center gap-2">
							{#if booked > 0}
								<span class="text-xs bg-blue-100 text-blue-800 px-1.5 py-0.5 rounded">{booked} bokad{booked > 1 ? 'e' : ''}</span>
							{/if}
							<span class="text-sm text-neutral-600">{group.available_count} kvar</span>
							<input
								type="number"
								min="1"
								max={group.available_count}
								bind:value={quantities[key]}
								class="border rounded w-14 px-2 py-1 text-sm text-center"
								placeholder="1"
							/>
							<button
								onclick={() => addToCart(group.commercial_name, group.location_name)}
								disabled={group.available_count === 0}
								class="bg-blue-700 text-white text-sm px-3 py-1 rounded disabled:opacity-50"
							>
								Lägg till
							</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}

		{#if cartItems.length > 0}
			<h2 class="font-medium mb-2">Din bokning ({cartItems.length} artiklar)</h2>
			<div class="border rounded mb-4">
				{#each cartGroups as group}
					<div class="px-4 py-2 border-b last:border-b-0">
						<div class="font-medium">{group.commercialName} × {group.items.length}</div>
						<div class="text-sm text-neutral-600 mt-1">
							{#each group.items as item}
								<div class="flex items-center justify-between py-0.5">
									<span>{item.common_name} — {item.location_name}{item.place ? `, ${item.place}` : ''}</span>
									<button onclick={() => removeFromCart(item.id)} class="text-red-600 text-xs hover:underline">Ta bort</button>
								</div>
							{/each}
						</div>
					</div>
				{/each}
			</div>

			<div class="flex gap-3">
				<button
					onclick={submitBooking}
					class="bg-green-700 text-white px-6 py-2 rounded font-medium"
				>
					Skicka bokning
				</button>
				<button
					onclick={cancelDraft}
					class="border border-red-300 text-red-700 px-4 py-2 rounded text-sm"
				>
					Avbryt
				</button>
			</div>
		{/if}
	{/if}
</div>
