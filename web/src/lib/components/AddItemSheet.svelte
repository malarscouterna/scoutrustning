
<script lang="ts">
	import { createApiClient, type AvailabilityGroup, type Category } from '$lib/api/client';

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
	let allGroups = $state<AvailabilityGroup[]>([]);
	let categories = $state<Category[]>([]);
	let selectedCategoryName = $state<string | null>(null);
	let loading = $state(false);
	let error = $state('');
	let quantities = $state<Record<string, number>>({});
	let adding = $state(false);

	function groupKey(g: AvailabilityGroup) {
		return `${g.commercial_name}|${g.location_name}`;
	}

	function isDisabled(g: AvailabilityGroup): boolean {
		return g.approval_level !== 'none' || (g.available_count + g.reported_usable_count) === 0;
	}

	function disabledReason(g: AvailabilityGroup): string {
		if (g.approval_level !== 'none') return 'Kräver godkännande';
		if ((g.available_count + g.reported_usable_count) === 0) return 'Ej tillgänglig';
		return '';
	}

	let results = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		return allGroups.filter((g) => {
			const matchesSearch = !query || g.commercial_name.toLowerCase().includes(query);
			const matchesCategory = !selectedCategoryName || g.category_name === selectedCategoryName;
			return matchesSearch && matchesCategory;
		});
	});

	let showResults = $derived(searchQuery.trim().length > 0 || selectedCategoryName !== null);

	async function loadAll() {
		loading = true;
		error = '';
		try {
			allGroups = await api.checkAvailability(startDate, endDate);
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadCategories() {
		try {
			categories = await api.listCategories();
		} catch {
			categories = [];
		}
	}

	loadAll();
	loadCategories();

	async function addItem(group: AvailabilityGroup) {
		if (isDisabled(group)) return;
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

<div class="fixed inset-0 z-50 flex items-start justify-center pt-12 px-4">
	<button type="button" class="absolute inset-0 bg-black/40" onclick={onClose} aria-label="Stäng"></button>
	<div class="relative bg-white rounded-2xl w-full max-w-lg max-h-[80vh] flex flex-col shadow-xl">
		<div class="flex items-center justify-between px-4 pt-4 pb-2 border-b">
			<h2 class="font-semibold text-base">Lägg till utrustning</h2>
			<button onclick={onClose} class="text-neutral-400 hover:text-neutral-600 text-xl leading-none">×</button>
		</div>

		<div class="px-4 pt-3 pb-2">
			<!-- svelte-ignore a11y_autofocus -->
			<input
				type="search"
				placeholder="Sök utrustning..."
				bind:value={searchQuery}
				class="w-full border rounded px-3 py-2 text-sm"
				autofocus
			/>
		</div>

		{#if categories.length > 0 && !searchQuery.trim()}
			<div class="px-4 pb-2 flex flex-wrap gap-1.5">
				{#each categories as cat}
					<button
						onclick={() => selectedCategoryName = selectedCategoryName === cat.name ? null : cat.name}
						class="text-xs px-3 py-1 rounded-full border transition-colors"
						class:bg-blue-700={selectedCategoryName === cat.name}
						class:text-white={selectedCategoryName === cat.name}
						class:border-blue-700={selectedCategoryName === cat.name}
					>{cat.name}</button>
				{/each}
			</div>
		{/if}

		{#if error}
			<div class="px-4 pb-2 text-red-600 text-sm">{error}</div>
		{/if}

		<div class="overflow-y-auto flex-1 px-4 pb-4 space-y-2">
			{#if loading}
				<p class="text-sm text-neutral-400 py-4 text-center">Laddar...</p>
			{:else if !showResults}
				<p class="text-sm text-neutral-400 py-4 text-center">Sök eller välj en kategori ovan.</p>
			{:else if results.length === 0}
				<p class="text-sm text-neutral-400 py-4 text-center">Inga artiklar hittades.</p>
			{:else}
				{#each results as group}
					{@const key = groupKey(group)}
					{@const qty = quantities[key] ?? 1}
					{@const disabled = isDisabled(group)}
					{@const reason = disabledReason(group)}
					<div class="border rounded p-3 flex items-center gap-3" class:opacity-50={disabled}>
						<div class="flex-1 min-w-0">
							<div class="font-medium text-sm">{group.commercial_name}</div>
							<div class="text-xs text-neutral-500">{group.location_name}</div>
							{#if reason}
								<div class="text-xs text-orange-600 mt-0.5">
									{reason}
									{#if reason === 'Kräver godkännande'}
										— <a href="/browse" class="underline">skapa ny bokning</a>
									{:else}
										— <a href="/browse" class="underline">välj annat datum</a>
									{/if}
								</div>
							{/if}
						</div>
						{#if !disabled}
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
						{/if}
					</div>
				{/each}
			{/if}
		</div>
	</div>
</div>
