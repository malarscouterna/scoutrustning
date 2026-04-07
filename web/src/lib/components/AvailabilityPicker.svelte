<script lang="ts">
	import { createApiClient, type AvailabilityGroup, type BookingItem, type Category, type Location } from '$lib/api/client';

	interface Props {
		bookingId: string;
		startDate: string;
		endDate: string;
		categories: Category[];
		locations: Location[];
		cartItems: BookingItem[];
		onItemsChanged: () => void;
	}

	let { bookingId, startDate, endDate, categories, locations, cartItems, onItemsChanged }: Props = $props();

	const api = createApiClient();

	function formatDate(iso: string): string {
		return new Date(iso).toLocaleDateString('sv', { day: 'numeric', month: 'short' });
	}

	let bookedCounts = $derived.by(() => {
		const map = new Map<string, number>();
		for (const item of cartItems) {
			const key = item.commercial_name + '||' + item.location_name;
			map.set(key, (map.get(key) ?? 0) + 1);
		}
		return map;
	});

	let stableAvailability = $state<AvailabilityGroup[]>([]);
	let selectedCategory = $state('');
	let selectedLocation = $state('');
	let searchQuery = $state('');
	let showRestricted = $state(false);
	let quantities = $state<Record<string, number>>({});
	let error = $state('');
	let expandedDetails = $state<string | null>(null);
	let detailArticles = $state<Map<string, { articles: any[]; comments: Map<string, string> }>>(new Map());

	let filteredAvailability = $derived(
		searchQuery
			? stableAvailability.filter(g => g.commercial_name.toLowerCase().includes(searchQuery.toLowerCase()))
			: stableAvailability
	);

	// Load on mount
	loadAvailability();

	async function toggleDetails(group: AvailabilityGroup) {
		const key = group.commercial_name + '||' + group.location_name;
		if (expandedDetails === key) {
			expandedDetails = null;
			return;
		}
		expandedDetails = key;
		if (detailArticles.has(key)) return;

		try {
			const articles = await api.listAvailableArticles(startDate, endDate, {
				exclude_booking_id: bookingId,
				commercial_name: group.commercial_name
			});
			const flagged = articles.filter(a => a.status !== 'ok');
			const comments = new Map<string, string>();

			await Promise.all(flagged.map(async (a) => {
				try {
					const { events } = await api.listArticleEvents(a.id, 1);
					if (events[0]?.description) comments.set(a.id, events[0].description);
				} catch { /* ignore */ }
			}));

			const next = new Map(detailArticles);
			next.set(key, { articles: flagged, comments });
			detailArticles = next;
		} catch { /* ignore */ }
	}

	export async function loadAvailability() {
		if (!startDate || !endDate) return;
		error = '';
		try {
			const fresh = await api.checkAvailability(startDate, endDate, {
				category_id: selectedCategory || undefined,
				location_id: selectedLocation || undefined,
				bookable_only: !showRestricted
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
		<input type="checkbox" bind:checked={showRestricted} onchange={onFilterChange} />
		Visa även låst utrustning
	</label>
</div>

<div class="space-y-1 mb-4">
	{#each filteredAvailability as group}
		{@const key = group.commercial_name + '||' + group.location_name}
		{@const hasFlags = group.reported_usable_count > 0 || group.incoming_count > 0 || group.under_repair_count > 0}
		{@const booked = bookedCounts.get(key) ?? 0}
		<div class="border rounded">
			<div class="flex flex-wrap items-center justify-between gap-2 px-4 py-2">
				<div class="min-w-0">
					<span class="font-medium">{group.commercial_name}</span>
					<span class="text-xs text-neutral-500 ml-2">{group.category_name} · {group.location_name}</span>
					{#if group.approval_level === 'low'}
						<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
					{:else if group.approval_level === 'high'}
						<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">Kräver särskilt godkännande</span>
					{/if}
				</div>
				<div class="flex items-center gap-2">
					<div class="text-sm text-neutral-600 text-right">
						{#if booked > 0}
							<span class="text-green-700 font-medium">{booked}/{booked + group.available_count} bokade</span>
						{:else}
							<span>{group.available_count} kvar</span>
						{/if}
						{#if hasFlags}
							<button onclick={() => toggleDetails(group)} class="text-xs text-neutral-400 ml-1 underline cursor-pointer">
								({#if group.reported_usable_count > 0}<span class="text-orange-600">{group.reported_usable_count} felrapp.</span>{/if}{#if group.incoming_count > 0}{#if group.reported_usable_count > 0}, {/if}<span class="text-blue-600">{group.incoming_count} inkommande</span>{/if}{#if group.under_repair_count > 0}{#if group.reported_usable_count > 0 || group.incoming_count > 0}, {/if}<span class="text-neutral-600">{group.under_repair_count} reparation</span>{/if})
							</button>
						{/if}
					</div>
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
			{#if expandedDetails === key}
				{@const detail = detailArticles.get(key)}
				<div class="border-t px-4 py-2 bg-neutral-50 text-sm space-y-1">
					{#if !detail}
						<p class="text-xs text-neutral-400">Laddar...</p>
					{:else if detail.articles.length === 0}
						<p class="text-xs text-neutral-400">Inga detaljer</p>
					{:else}
						{#each detail.articles as article}
							<div class="flex flex-wrap items-start gap-2 text-xs">
								<span class="px-1.5 py-0.5 rounded
									{article.status === 'reported_usable' ? 'bg-orange-100 text-orange-700' : article.status === 'incoming' ? 'bg-blue-50 text-blue-700' : article.status === 'under_repair' ? 'bg-neutral-100 text-neutral-700' : 'bg-neutral-100'}">
									{article.status === 'reported_usable' ? 'Felrapporterad' : article.status === 'incoming' ? 'Inkommande' : article.status === 'under_repair' ? 'Under reparation' : article.status}
								</span>
								{#if article.common_name !== group.commercial_name}
									<span class="text-neutral-600">{article.common_name}</span>
								{/if}
								{#if article.expected_available_date}
									<span class="text-blue-600">{article.status === 'incoming' ? 'beräknas levereras' : 'beräknas klar'} {formatDate(article.expected_available_date)}</span>
								{/if}
								{#if detail.comments.get(article.id)}
									<span class="text-neutral-500 italic">“{detail.comments.get(article.id)}”</span>
								{/if}
							</div>
						{/each}
					{/if}
				</div>
			{/if}
		</div>
	{/each}
</div>
