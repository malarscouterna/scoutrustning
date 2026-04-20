<script lang="ts">
	import { createApiClient, type AvailabilityGroup } from '$lib/api/client';

	interface Props {
		bookingId: string;
		startDate: string;
		endDate: string;
		onAdded: () => Promise<void>;
		onClose: () => void;
	}

	let { bookingId, startDate, endDate, onAdded, onClose }: Props = $props();

	const api = createApiClient();

	let searchQuery = $state('');
	let results = $state<AvailabilityGroup[]>([]);
	let loading = $state(false);
	let error = $state('');
	let quantities = $state<Record<string, number>>({});
	let adding = $state(false);

	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	function groupKey(g: AvailabilityGroup) {
		return `${g.commercial_name}|${g.location_name}`;
	}

	async function search(query: string) {
		if (!query.trim()) { results = []; return; }
		loading = true;
		error = '';
		try {
			const all = await api.checkAvailability(startDate, endDate);
			results = all.filter((g) =>
				g.commercial_name.toLowerCase().includes(query.toLowerCase()) &&
				(g.available_count + g.reported_usable_count) > 0
			);
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function onSearchInput(query: string) {
		searchQuery = query;
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => search(query), 300);
	}

	async function addItem(group: AvailabilityGroup) {
		const key = groupKey(group);
		const qty = quantities[key] ?? 1;
		if (qty < 1) return;
		adding = true;
		error = '';
		try {
			await api.addBookingItems(bookingId, group.commercial_name, qty, group.location_name);
			await onAdded();
		} catch (e: any) {
			error = e.message;
			adding = false;
		}
	}
</script>

<div class="fixed inset-0 z-50 flex flex-col justify-end">
	<button type="button" class="absolute inset-0 bg-black/40" onclick={onClose} aria-label="Stäng"></button>
	<div class="relative bg-white rounded-t-2xl max-h-[85vh] flex flex-col shadow-xl">
		<div class="flex items-center justify-between px-4 pt-4 pb-2 border-b">
			<h2 class="font-semibold text-base">Lägg till utrustning</h2>
			<button onclick={onClose} class="text-neutral-400 hover:text-neutral-600 text-xl leading-none">×</button>
		</div>

		<div class="px-4 py-3">
			<input
				type="search"
				placeholder="Sök utrustning..."
				value={searchQuery}
				oninput={(e) => onSearchInput(e.currentTarget.value)}
				class="w-full border rounded px-3 py-2 text-sm"
				autofocus
			/>
		</div>

		{#if error}
			<div class="px-4 pb-2 text-red-600 text-sm">{error}</div>
		{/if}

		<div class="overflow-y-auto flex-1 px-4 pb-4 space-y-2">
			{#if loading}
				<p class="text-sm text-neutral-400 py-4 text-center">Söker...</p>
			{:else if results.length === 0 && searchQuery.trim()}
				<p class="text-sm text-neutral-400 py-4 text-center">Inga tillgängliga artiklar hittades.</p>
			{:else}
				{#each results as group}
					{@const key = groupKey(group)}
					{@const qty = quantities[key] ?? 1}
					{@const maxAvail = group.available_count + group.reported_usable_count}
					<div class="border rounded p-3 flex items-center gap-3">
						<div class="flex-1 min-w-0">
							<div class="font-medium text-sm">{group.commercial_name}</div>
							<div class="text-xs text-neutral-500">{group.location_name} · {maxAvail} tillgängliga</div>
							{#if group.approval_level !== 'none'}
								<div class="text-xs text-orange-600 mt-0.5">Kan kräva godkännande</div>
							{/if}
						</div>
						<div class="flex items-center gap-1">
							<button
								onclick={() => quantities[key] = Math.max(1, qty - 1)}
								class="w-7 h-7 border rounded text-sm flex items-center justify-center"
								aria-label="Minska antal"
							>−</button>
							<span class="w-8 text-center text-sm">{qty}</span>
							<button
								onclick={() => quantities[key] = qty + 1}
								class="w-7 h-7 border rounded text-sm flex items-center justify-center"
								aria-label="Öka antal"
							>+</button>
						</div>
						<button
							onclick={() => addItem(group)}
							disabled={adding}
							class="text-xs bg-blue-700 text-white px-3 py-1.5 rounded disabled:opacity-50"
						>Lägg till</button>
					</div>
				{/each}
			{/if}
		</div>
	</div>
</div>
