<script lang="ts">
	import { cart } from '$lib/stores/cart.svelte';
	import { createApiClient, type Booking, type BookingItem } from '$lib/api/client';
	import { goto } from '$app/navigation';

	const api = createApiClient();

	let open = $state(false);
	let booking = $state<Booking | null>(null);
	let items = $state<BookingItem[]>([]);
	let loading = $state(false);
	let submitting = $state(false);
	let error = $state('');

	interface ItemGroup {
		commercialName: string;
		locationName: string;
		categoryName: string;
		articleId: string;
		count: number;
		itemIds: string[];
	}

	interface LocationSection {
		locationName: string;
		groups: ItemGroup[];
	}

	let grouped = $derived.by(() => {
		const map = new Map<string, ItemGroup>();
		for (const item of items) {
			const key = item.commercial_name + '||' + item.location_name;
			const g = map.get(key);
			if (g) {
				g.count++;
				g.itemIds.push(item.id);
			} else {
				map.set(key, {
					commercialName: item.commercial_name,
					locationName: item.location_name,
					categoryName: item.category_name,
					articleId: item.article_id,
					count: 1,
					itemIds: [item.id]
				});
			}
		}
		return [...map.values()];
	});

	let locationSections = $derived.by(() => {
		const sectionMap = new Map<string, LocationSection>();
		const sorted = [...grouped].sort((a, b) =>
			a.categoryName.localeCompare(b.categoryName, 'sv') ||
			a.commercialName.localeCompare(b.commercialName, 'sv')
		);
		for (const g of sorted) {
			const existing = sectionMap.get(g.locationName);
			if (existing) {
				existing.groups.push(g);
			} else {
				sectionMap.set(g.locationName, { locationName: g.locationName, groups: [g] });
			}
		}
		return [...sectionMap.values()];
	});

	let itemCount = $derived(items.length);

	// Badge animation
	let prevCount = 0;
	let badgeAnim = $state('');
	$effect(() => {
		const count = itemCount;
		if (count === 0) { prevCount = 0; badgeAnim = ''; return; }
		if (prevCount === 0 && count > 0) {
			badgeAnim = 'badge-pop';
		} else if (count > prevCount) {
			badgeAnim = 'badge-bump';
		}
		prevCount = count;
		const timer = setTimeout(() => { badgeAnim = ''; }, 400);
		return () => clearTimeout(timer);
	});

	function formatDateShort(iso: string): string {
		return new Date(iso).toLocaleDateString('sv', { day: 'numeric', month: 'short' });
	}

	async function loadSilent() {
		if (!cart.id) return;
		try {
			const result = await api.getBooking(cart.id);
			booking = result.booking;
			items = result.items;
			if (booking.status !== 'draft') {
				cart.clear();
				booking = null;
				items = [];
				open = false;
			}
		} catch {
			cart.clear();
			booking = null;
			items = [];
		}
	}

	async function load() {
		if (!cart.id) return;
		loading = true;
		error = '';
		await loadSilent();
		loading = false;
	}

	$effect(() => {
		const id = cart.id;
		if (id) {
			loadSilent();
		} else {
			booking = null;
			items = [];
			open = false;
		}
	});

	// Refresh when cart is modified externally (e.g., items added from browse page)
	$effect(() => {
		const _ = cart.refreshSignal; // read the signal
		if (cart.id) {
			loadSilent();
		}
	});

	async function toggle() {
		if (!open && cart.id) await load();
		open = !open;
	}

	function close() {
		open = false;
	}

	async function addOne(commercialName: string, locationName: string) {
		if (!cart.id) return;
		error = '';
		try {
			await api.addBookingItems(cart.id, commercialName, 1, locationName);
			await loadSilent();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function removeOne(group: ItemGroup) {
		if (!cart.id) return;
		error = '';
		// Find items in this group and sort by status to remove low-priority items first
		const itemsInGroup = items.filter(i => i.commercial_name === group.commercialName && i.location_name === group.locationName);
		const statusPriority: Record<string, number> = {
			'reported_usable': 0,
			'under_repair': 1,
			'incoming': 2,
			'ok': 3
		};
		itemsInGroup.sort((a, b) => (statusPriority[a.article_status] ?? 4) - (statusPriority[b.article_status] ?? 4));
		const itemId = itemsInGroup[0]?.id;
		if (!itemId) return;
		try {
			await api.removeBookingItem(cart.id, itemId);
			await loadSilent();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function removeGroup(group: ItemGroup) {
		if (!cart.id) return;
		error = '';
		try {
			for (const id of group.itemIds) {
				await api.removeBookingItem(cart.id, id);
			}
			await loadSilent();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function submit() {
		if (!cart.id) return;
		submitting = true;
		error = '';
		try {
			await api.submitBooking(cart.id);
			const id = cart.id;
			cart.clear();
			open = false;
			goto(`/bookings/${id}`);
		} catch (e: any) {
			error = e.message;
		}
		submitting = false;
	}
</script>

{#if cart.active}
	<!-- Mobile backdrop only -->
	{#if open}
		<button class="fixed inset-0 z-40 bg-black/20 md:hidden" onclick={close} aria-label="Stäng varukorg"></button>
	{/if}

	<div class="fixed bottom-4 right-4 z-50">
		{#if open}
			<div class="bg-white border rounded-lg shadow-xl w-80 sm:w-96 mb-2 max-h-[60vh] flex flex-col">
				{#if loading}
					<div class="p-4 text-sm text-neutral-500">Laddar...</div>
				{:else if booking}
					<div class="px-4 py-2 border-b text-sm font-medium text-neutral-700 shrink-0 flex items-center justify-between gap-2">
						<span>
							{formatDateShort(booking.start_date)} - {formatDateShort(booking.end_date)}
							{#if booking.team_name}· {booking.team_name}{/if}
						</span>
						<button onclick={close} class="text-neutral-400 hover:text-neutral-600 text-lg leading-none shrink-0">✕</button>
					</div>

					{#if error}
						<div class="px-4 py-2 text-xs text-red-700 bg-red-50 border-b">{error}</div>
					{/if}

					<div class="overflow-y-auto flex-1">
						{#if locationSections.length === 0}
							<p class="px-4 py-3 text-sm text-neutral-500">Inga artiklar ännu.</p>
						{:else}
							{#each locationSections as section}
								<div class="px-4 pt-2 pb-0.5 text-xs font-semibold text-neutral-500 uppercase tracking-wide">{section.locationName}</div>
								{#each section.groups as group}
									<div class="flex items-center gap-2 px-4 py-2 border-b last:border-b-0 text-sm">
										<div class="flex-1 min-w-0">
											<a
												href="/articles/{group.articleId}"
												onclick={close}
												class="font-medium truncate block text-blue-700 hover:underline"
											>{group.commercialName}</a>
											<span class="text-xs text-neutral-500">{group.categoryName}</span>
										</div>
										<div class="flex items-center gap-1 shrink-0">
											<button onclick={() => removeOne(group)} class="w-7 h-7 rounded border text-center text-sm hover:bg-neutral-50">−</button>
											<span class="w-6 text-center text-sm font-medium">{group.count}</span>
											<button onclick={() => addOne(group.commercialName, group.locationName)} class="w-7 h-7 rounded border text-center text-sm hover:bg-neutral-50">+</button>
											<button onclick={() => removeGroup(group)} class="w-7 h-7 rounded text-neutral-400 hover:text-red-600 text-center">×</button>
										</div>
									</div>
								{/each}
							{/each}
						{/if}
					</div>

					<div class="flex items-center gap-2 px-4 py-3 border-t shrink-0">
						<a href="/book?id={cart.id}" class="text-sm text-blue-700 hover:underline">Visa bokning ›</a>
						<button
							onclick={submit}
							disabled={itemCount === 0 || submitting}
							class="ml-auto bg-blue-700 text-white text-sm px-4 py-1.5 rounded disabled:opacity-50"
						>
							{submitting ? '...' : 'Skicka'} ›
						</button>
					</div>
				{/if}
			</div>
		{/if}

		<!-- FAB -->
		<button
			onclick={toggle}
			class="relative flex items-center justify-center gap-2 bg-blue-700 text-white shadow-lg hover:bg-blue-800 ml-auto
				w-14 h-14 rounded-full
				md:w-auto md:h-auto md:rounded-full md:px-5 md:py-3"
			aria-label="Min bokning"
		>
			<span class="material-symbols-outlined" style="font-size:22px">shopping_bag</span>
			<span class="hidden md:inline text-sm font-medium whitespace-nowrap">
				Min bokning{itemCount > 0 ? ` (${itemCount})` : ''}
			</span>
			{#if itemCount > 0}
				<span
					class="absolute -top-1.5 -right-1.5 bg-red-600 text-white text-xs rounded-full w-6 h-6 flex items-center justify-center font-bold leading-none {badgeAnim}"
				>
					{itemCount}
				</span>
			{/if}
		</button>
	</div>
{/if}

<style>
	@keyframes badge-pop {
		0%   { transform: scale(0.4); opacity: 0; }
		60%  { transform: scale(1.25); }
		100% { transform: scale(1); opacity: 1; }
	}
	@keyframes badge-bump {
		0%   { transform: scale(1); }
		30%  { transform: scale(1.45); }
		65%  { transform: scale(0.9); }
		100% { transform: scale(1); }
	}
	.badge-pop  { animation: badge-pop  0.35s ease-out; }
	.badge-bump { animation: badge-bump 0.28s ease-out; }
</style>
