<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import AvailabilityPicker from '$lib/components/AvailabilityPicker.svelte';
	import BookingItemsList from '$lib/components/BookingItemsList.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const api = createApiClient();

	// Filter units/projects to those the user is a member of (managers see all)
	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let userUnits = $derived(
		isManager
			? data.units
			: data.units.filter(u => ($page.data.user?.units ?? []).includes(u.name))
	);

	let isEdit = $derived(!!data.existing);

	let startDate = $state(data.existing?.booking.start_date ?? '');
	let endDate = $state(data.existing?.booking.end_date ?? '');
	let notes = $state(data.existing?.booking.notes ?? '');
	let defaultUnit = $derived.by(() => {
		const myUnitNames = new Set($page.data.user?.units ?? []);
		const match = data.units.find(u => myUnitNames.has(u.name));
		return match?.id ?? '';
	});
	let selectedUnit = $state(data.existing?.booking.used_by_unit_id ?? '');
	let unitInitialized = $state(!!data.existing);
	$effect(() => {
		if (!unitInitialized && defaultUnit) {
			selectedUnit = defaultUnit;
			unitInitialized = true;
		}
	});
	let bookingId = $state<string | null>(data.existing?.booking.id ?? null);
	let cartItems = $state<BookingItem[]>(data.existing?.items ?? []);
	let error = $state('');
	let message = $state('');
	let submitted = $state(false);
	let loading = $state(false);
	let showAvailability = $state(!!data.existing);

	let cartEl = $state<HTMLElement | undefined>(undefined);

	function showMessage(msg: string) {
		message = msg;
		setTimeout(() => message = '', 4000);
	}

	function scrollToCart() {
		cartEl?.scrollIntoView({ behavior: 'smooth' });
	}

	async function openAvailability() {
		if (!startDate || !endDate) return;
		error = '';
		loading = true;
		try {
			if (!bookingId) {
				const booking = await api.createBooking({
					start_date: startDate,
					end_date: endDate,
					notes,
					used_by_unit_id: selectedUnit || undefined
				});
				bookingId = booking.id;
			}
			showAvailability = true;
		} catch (e: any) {
			error = e.message;
		}
		loading = false;
	}

	async function saveDetails() {
		if (!bookingId) return;
		error = '';
		try {
			await api.updateBooking(bookingId, {
				start_date: startDate,
				end_date: endDate,
				notes,
				used_by_unit_id: selectedUnit || null
			});
			showMessage('Ändringar sparade');
		} catch (e: any) {
			error = e.message;
		}
	}

	let prevCartCount = $state(cartItems.length);

	async function refreshCart() {
		if (!bookingId) return;
		const result = await api.getBooking(bookingId);
		cartItems = result.items;
		prevCartCount = result.items.length;
	}

	async function removeFromCart(itemId: string) {
		if (!bookingId) return;
		error = '';
		try {
			await api.removeBookingItem(bookingId, itemId);
			const result = await api.getBooking(bookingId);
			cartItems = result.items;
		} catch (e: any) {
			error = e.message;
		}
	}

	async function submitBooking() {
		if (!bookingId) return;
		error = '';
		try {
			const booking = await api.submitBooking(bookingId);
			submitted = true;
			message = booking.status === 'confirmed' ? 'Bokning bekräftad!' : 'Bokning inskickad!';
		} catch (e: any) {
			error = e.message;
		}
	}

	async function cancelBooking() {
		if (!bookingId) return;
		if (!confirm('Är du säker?')) return;
		try {
			await api.cancelBooking(bookingId);
			window.location.href = '/bookings';
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

<div class="max-w-4xl mx-auto p-4 pb-20">
	<h1 class="text-heading-sm font-bold mb-4">
		{isEdit ? 'Redigera bokning' : 'Boka utrustning'}
	</h1>

	{#if submitted}
		<div class="bg-green-50 border border-green-200 rounded p-4">
			<p class="font-medium text-green-800">{message}</p>
			<a href="/bookings/{bookingId}" class="underline text-green-700">Visa bokning →</a>
		</div>
	{:else}
		{#if message}
			<div class="bg-green-50 border border-green-200 rounded p-3 mb-4 text-green-800 text-sm">{message}</div>
		{/if}

		{#if error}
			<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{error}</div>
		{/if}

		<!-- 1. Booking details -->
		<div class="flex flex-wrap gap-3 mb-4">
			<label class="flex flex-col gap-1">
				<span class="text-sm">Startdatum</span>
				<input type="date" bind:value={startDate} disabled={cartItems.length > 0} class="border rounded px-3 py-2 disabled:opacity-50" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">Slutdatum</span>
				<input type="date" bind:value={endDate} disabled={cartItems.length > 0} class="border rounded px-3 py-2 disabled:opacity-50" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">Anteckningar</span>
				<input type="text" bind:value={notes} placeholder="T.ex. Hajk med Yggdrasil" class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">Bokas för</span>
				<select bind:value={selectedUnit} class="border rounded px-3 py-2">
					<option value="">Personlig bokning</option>
					{#each userUnits as unit}
						<option value={unit.id}>{unit.name}{unit.type === 'project' ? ' (projekt)' : ''}</option>
					{/each}
				</select>
			</label>
		</div>

		{#if isEdit}
			<div class="flex gap-2 mb-4">
				<button onclick={saveDetails} class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Spara ändringar</button>
				<a href="/bookings/{bookingId}" class="border rounded px-4 py-2 text-sm flex items-center">Tillbaka</a>
			</div>
		{/if}

		<!-- 2. Availability picker -->
		<div class="mb-6">
			{#if showAvailability}
				<div class="flex items-center justify-between mb-2">
					<h2 class="font-medium">Lägg till utrustning</h2>
					<button onclick={() => showAvailability = false} class="text-sm text-neutral-500 underline">Dölj</button>
				</div>
				{#if bookingId}
					<AvailabilityPicker
						{bookingId}
						{startDate}
						{endDate}
						categories={data.categories}
						locations={data.locations}
						{cartItems}
						onItemsChanged={refreshCart}
					/>
				{/if}
			{:else}
				<button
					onclick={openAvailability}
					disabled={!startDate || !endDate || loading}
					class="bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50"
				>
					{loading ? '...' : 'Lägg till utrustning'}
				</button>
			{/if}
		</div>

		<!-- 3. Cart (below picker so adding doesn't shift the view) -->
		<div bind:this={cartEl}>
			{#if cartItems.length > 0}
				<h2 class="font-medium mb-2">{isEdit ? 'Utrustning' : 'Din bokning'} ({cartItems.length} artiklar)</h2>
				<BookingItemsList items={cartItems} editable onRemove={removeFromCart} />
			{/if}
		</div>

		<!-- 4. Sticky bottom bar -->
		{#if bookingId}
			<div class="fixed bottom-0 left-0 right-0 bg-white border-t px-4 py-3 z-10">
				<div class="max-w-4xl mx-auto flex items-center gap-3">
					<button
						onclick={submitBooking}
						disabled={cartItems.length === 0}
						class="bg-green-700 text-white px-5 py-2 rounded font-medium disabled:opacity-50"
					>
						{isEdit ? 'Spara och skicka' : 'Skicka bokning'} ({cartItems.length})
					</button>
					{#if cartItems.length > 0}
						<button onclick={scrollToCart} class="border rounded px-4 py-2 text-sm">
							Visa vald utrustning ↓
						</button>
					{/if}
					<button onclick={cancelBooking} class="text-sm text-red-600 underline ml-auto">
						{isEdit ? 'Avboka' : 'Avbryt'}
					</button>
				</div>
			</div>
		{/if}
	{/if}
</div>
