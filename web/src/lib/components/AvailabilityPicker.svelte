<script lang="ts">
	import { createApiClient, type AvailabilityGroup, type Category, type Location } from '$lib/api/client';

	interface Props {
		bookingId: string;
		startDate: string;
		endDate: string;
		categories: Category[];
		locations: Location[];
		onItemsChanged: () => void;
	}

	let { bookingId, startDate, endDate, categories, locations, onItemsChanged }: Props = $props();

	const api = createApiClient({ persona: 'leader-yggdrasil' });

	let stableAvailability = $state<AvailabilityGroup[]>([]);
	let selectedCategory = $state('');
	let selectedLocation = $state('');
	let searchQuery = $state('');
	let showRequiresApproval = $state(false);
	let quantities = $state<Record<string, number>>({});
	let error = $state('');

	let filteredAvailability = $derived(
		searchQuery
			? stableAvailability.filter(g => g.commercial_name.toLowerCase().includes(searchQuery.toLowerCase()))
			: stableAvailability
	);

	// Load on mount
	loadAvailability();

	export async function loadAvailability() {
		if (!startDate || !endDate) return;
		error = '';
		try {
			const fresh = await api.checkAvailability(startDate, endDate, {
				category_id: selectedCategory || undefined,
				location_id: selectedLocation || undefined,
				bookable_only: !showRequiresApproval
			});
			if (stableAvailability.length === 0) {
				stableAvailability = fresh;
			} else {
				const freshMap = new Map<string, AvailabilityGroup>();
				for (const g of fresh) {
					freshMap.set(g.commercial_name + '||' + g.location_name, g);
				}
				stableAvailability = stableAvailability.map(g => {
					const key = g.commercial_name + '||' + g.location_name;
					return freshMap.get(key) ?? { ...g, available_count: 0 };
				});
			}
		} catch (e: any) {
			error = e.message;
		}
	}

	function onFilterChange() {
		stableAvailability = [];
		loadAvailability();
	}

	async function addToCart(commercialName: string, locationName: string) {
		const key = commercialName + '||' + locationName;
		const qty = quantities[key] || 1;
		error = '';
		try {
			await api.addBookingItems(bookingId, commercialName, qty, locationName);
			quantities[key] = 1;
			await loadAvailability();
			onItemsChanged();
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

{#if error}
	<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{error}</div>
{/if}

<div class="flex flex-wrap gap-2 mb-3">
	<input
		type="search"
		placeholder="Sök..."
		bind:value={searchQuery}
		class="border rounded px-3 py-2 text-sm flex-1 min-w-48"
	/>
	<select bind:value={selectedCategory} onchange={onFilterChange} class="border rounded px-3 py-2 text-sm">
		<option value="">Alla kategorier</option>
		{#each categories as cat}
			<option value={cat.id}>{cat.name}</option>
		{/each}
	</select>
	<select bind:value={selectedLocation} onchange={onFilterChange} class="border rounded px-3 py-2 text-sm">
		<option value="">Alla platser</option>
		{#each locations as loc}
			<option value={loc.id}>{loc.name}</option>
		{/each}
	</select>
	<label class="flex items-center gap-1.5 text-sm">
		<input type="checkbox" bind:checked={showRequiresApproval} onchange={onFilterChange} />
		Visa även låst utrustning
	</label>
</div>

<div class="space-y-1 mb-4">
	{#each filteredAvailability as group}
		{@const key = group.commercial_name + '||' + group.location_name}
		<div class="flex items-center justify-between border rounded px-4 py-2">
			<div>
				<span class="font-medium">{group.commercial_name}</span>
				<span class="text-xs text-neutral-500 ml-2">{group.category_name} · {group.location_name}</span>
				{#if group.requires_approval}
					<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
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
