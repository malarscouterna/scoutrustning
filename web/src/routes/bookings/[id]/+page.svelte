<script lang="ts">
	import { createApiClient, type Booking, type BookingItem } from '$lib/api/client';
	import BookingItemsList from '$lib/components/BookingItemsList.svelte';
	import PickupChecklist from '$lib/components/PickupChecklist.svelte';
	import type { PageData } from './$types';
	import { page } from '$app/stores';

	let { data }: { data: PageData } = $props();

	const api = createApiClient({ persona: 'leader-yggdrasil' });

	let booking = $state<Booking>(data.booking);
	let items = $state<BookingItem[]>(data.items);
	let error = $state('');
	let message = $state($page.url.searchParams.get('msg') ?? '');

	if (message) {
		history.replaceState({}, '', $page.url.pathname);
		setTimeout(() => message = '', 4000);
	}

	const statusLabels: Record<string, string> = {
		draft: 'Utkast',
		submitted: 'Inskickad',
		approved: 'Godkänd',
		confirmed: 'Bekräftad',
		picked_up: 'Uthämtad',
		returned: 'Återlämnad',
		rejected: 'Nekad',
		cancelled: 'Avbokad'
	};

	const statusColors: Record<string, string> = {
		draft: 'bg-neutral-100',
		submitted: 'bg-orange-100 text-orange-800',
		confirmed: 'bg-green-100 text-green-800',
		picked_up: 'bg-blue-100 text-blue-800',
		returned: 'bg-neutral-100',
		rejected: 'bg-red-100 text-red-800',
		cancelled: 'bg-neutral-100 text-neutral-500'
	};

	let editable = $derived(
		['draft', 'submitted', 'approved', 'confirmed', 'picked_up'].includes(booking.status)
	);

	let cancellable = $derived(
		booking.status !== 'returned' && booking.status !== 'cancelled'
	);

	async function submitBooking() {
		error = '';
		try {
			booking = await api.submitBooking(booking.id);
			message = booking.status === 'confirmed' ? 'Bokning bekräftad' : 'Bokning inskickad';
			setTimeout(() => message = '', 4000);
		} catch (e: any) {
			error = e.message;
		}
	}

	async function cancelBooking() {
		if (!confirm('Är du säker på att du vill avboka?')) return;
		error = '';
		try {
			await api.cancelBooking(booking.id);
			if (booking.status === 'draft') {
				window.location.href = '/bookings';
			} else {
				booking = { ...booking, status: 'cancelled' };
			}
		} catch (e: any) {
			error = e.message;
		}
	}

	async function startPickup() {
		error = '';
		try {
			booking = await api.pickupBooking(booking.id);
			message = 'Utlämning startad';
			setTimeout(() => message = '', 4000);
		} catch (e: any) {
			error = e.message;
		}
	}

	function handlePickupUpdate(updatedItems: BookingItem[]) {
		items = updatedItems;
	}

</script>

<div class="max-w-4xl mx-auto p-4">
	<a href="/bookings" class="text-sm text-blue-700 underline">← Tillbaka</a>

	{#if message}
		<div class="bg-green-50 border border-green-200 rounded p-3 mt-4 text-green-800 text-sm">{message}</div>
	{/if}

	{#if error}
		<div class="bg-red-50 border border-red-200 rounded p-3 mt-4 text-red-800 text-sm">{error}</div>
	{/if}

	<div class="mt-4 mb-6">
		<div class="flex items-center gap-3 mb-2">
			<h1 class="text-heading-sm font-bold">
				{booking.start_date} — {booking.end_date}
			</h1>
			<span class="text-sm px-2 py-0.5 rounded {statusColors[booking.status] ?? 'bg-neutral-100'}">
				{statusLabels[booking.status] ?? booking.status}
			</span>
		</div>
		{#if booking.notes}
			<p class="text-neutral-600 mb-2">{booking.notes}</p>
		{/if}
		<p class="text-sm text-neutral-500 mb-3">
			{#if booking.unit_name}
				För: <span class="font-medium text-blue-700">{booking.unit_name}</span>
			{:else if booking.used_by_external}
				För: {booking.used_by_external}
			{:else}
				Personlig bokning
			{/if}
		</p>

		<div class="flex items-center gap-2">
			{#if editable}
				<a href="/book?id={booking.id}" class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Redigera</a>
			{/if}
			{#if booking.status === 'draft'}
				<button onclick={submitBooking} class="bg-green-700 text-white px-4 py-2 rounded text-sm">Skicka bokning</button>
			{/if}
			{#if booking.status === 'confirmed' || booking.status === 'approved'}
				<button onclick={startPickup} class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Starta utlämning</button>
			{/if}
			{#if cancellable}
				<button onclick={cancelBooking} class="text-sm text-red-600 underline">
					{booking.status === 'draft' ? 'Ta bort utkast' : 'Avboka'}
				</button>
			{/if}
		</div>
	</div>

	<h2 class="font-medium mb-2">Utrustning ({items.length} artiklar)</h2>
	{#if booking.status === 'picked_up'}
		<PickupChecklist
			bookingId={booking.id}
			{items}
			startDate={booking.start_date}
			endDate={booking.end_date}
			onUpdate={handlePickupUpdate}
		/>
	{:else}
		<BookingItemsList {items} />
	{/if}
</div>
