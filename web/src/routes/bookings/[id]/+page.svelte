<script lang="ts">
	import { createApiClient, type Booking, type BookingItem, type BookingEvent } from '$lib/api/client';
	import BookingItemsList from '$lib/components/BookingItemsList.svelte';
	import PickupChecklist from '$lib/components/PickupChecklist.svelte';
	import ReturnChecklist from '$lib/components/ReturnChecklist.svelte';
	import type { PageData } from './$types';
	import { page } from '$app/stores';
	import { onDestroy } from 'svelte';

	let { data }: { data: PageData } = $props();

	const api = createApiClient();

	let booking = $state<Booking>(data.booking);
	let items = $state<BookingItem[]>(data.items);
	let error = $state('');
	let message = $state($page.url.searchParams.get('msg') ?? '');

	if (message) {
		history.replaceState({}, '', $page.url.pathname);
		setTimeout(() => message = '', 4000);
	}

	// Poll for updates during active pickup/return
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	function startPolling() {
		stopPolling();
		pollTimer = setInterval(async () => {
			try {
				const result = await api.getBooking(booking.id);
				items = result.items;
				if (result.booking.status !== booking.status) {
					booking = result.booking;
				}
			} catch { /* ignore poll errors */ }
		}, 10_000);
	}

	function stopPolling() {
		if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
	}

	$effect(() => {
		const active = !['returned', 'cancelled', 'rejected'].includes(booking.status);
		if (active) {
			startPolling();
		} else {
			stopPolling();
		}
	});

	onDestroy(stopPolling);

	if (message) {
		history.replaceState({}, '', $page.url.pathname);
		setTimeout(() => message = '', 4000);
	}

	const statusLabels: Record<string, string> = {
		draft: 'Utkast',
		submitted: 'Väntar på godkännande',
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
			booking = await api.submitBooking(booking.id, submitMessage || undefined, forceApproval || undefined);
			message = booking.status === 'confirmed' ? 'Bokning bekräftad' : 'Bokning inskickad';
			submitMessage = '';
			forceApproval = false;
			setTimeout(() => message = '', 4000);
			loadEvents();
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

	function handleReturnUpdate(updatedItems: BookingItem[]) {
		items = updatedItems;
	}

	function handleBookingReturned() {
		booking = { ...booking, status: 'returned' };
		message = 'Allt återlämnat!';
		setTimeout(() => message = '', 4000);
	}

	async function reopenBooking() {
		error = '';
		try {
			booking = { ...booking, status: 'picked_up' };
			showReturn = true;
			message = 'Bokning öppnad igen';
			setTimeout(() => message = '', 4000);
		} catch (e: any) {
			error = e.message;
		}
	}

	let anyPickedUp = $derived(
		items.some((i) => i.pickup_status !== null)
	);

	let showReturn = $state(false);
	let approvalMessage = $state('');
	let submitMessage = $state('');
	let forceApproval = $state(false);
	let isManager = $derived(data.user?.roles.includes('equipment_manager') ?? false);
	let bookingEvents = $state<BookingEvent[]>([]);

	async function loadEvents() {
		try {
			bookingEvents = await api.listBookingEvents(booking.id);
		} catch { /* ignore */ }
	}
	loadEvents();

	let noteMessage = $state('');

	async function addNote() {
		if (!noteMessage.trim()) return;
		error = '';
		try {
			await api.addBookingNote(booking.id, noteMessage);
			noteMessage = '';
			loadEvents();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function approveBooking() {
		error = '';
		try {
			booking = await api.approveBooking(booking.id, approvalMessage);
			message = 'Bokning godkänd';
			approvalMessage = '';
			setTimeout(() => message = '', 4000);
			loadEvents();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function rejectBooking() {
		error = '';
		try {
			booking = await api.rejectBooking(booking.id, approvalMessage);
			message = 'Bokning nekad';
			approvalMessage = '';
			setTimeout(() => message = '', 4000);
			loadEvents();
		} catch (e: any) {
			error = e.message;
		}
	}

	// Auto-resume return mode if a return was already started
	$effect(() => {
		if (items.some((i) => i.return_status && i.return_status !== 'pending')) {
			showReturn = true;
		}
	});

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

		{#if bookingEvents.length > 0}
			<div class="border rounded mb-3 divide-y">
				{#each bookingEvents as event}
					<div class="px-4 py-2 text-sm {event.event_type === 'rejected' ? 'bg-red-50' : event.event_type === 'approved' ? 'bg-green-50' : 'bg-neutral-50'}">
						<div class="flex items-center gap-2 text-xs text-neutral-500 mb-0.5">
							<span class="font-medium text-neutral-700">{event.actor_name}</span>
							<span>
								{{
									submitted: 'skickade för godkännande',
									approved: 'godkände',
									rejected: 'nekade',
									cancelled: 'avbokade',
									note: 'kommenterade'
								}[event.event_type] ?? event.event_type}
							</span>
							<span>{new Date(event.created_at).toLocaleDateString('sv', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span>
						</div>
						{#if event.message}
							<p class="text-neutral-700">{event.message}</p>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<div class="flex gap-2 mb-3">
			<input
				type="text"
				bind:value={noteMessage}
				placeholder="Lägg till en kommentar..."
				class="flex-1 border rounded px-3 py-2 text-sm"
				onkeydown={(e) => { if (e.key === 'Enter') addNote(); }}
			/>
			<button onclick={addNote} disabled={!noteMessage.trim()} class="bg-neutral-700 text-white px-3 py-2 rounded text-sm disabled:opacity-50">Skicka</button>
		</div>

		{#if isManager && booking.status === 'submitted'}
			<div class="border rounded p-4 mb-3 bg-orange-50">
				<p class="text-sm font-medium text-orange-800 mb-2">Denna bokning väntar på godkännande</p>
				<textarea
					bind:value={approvalMessage}
					placeholder="Meddelande till bokaren (valfritt)"
					class="w-full border rounded px-3 py-2 text-sm mb-2"
					rows="2"
				></textarea>
				<div class="flex gap-2">
					<button onclick={approveBooking} class="bg-green-700 text-white px-4 py-2 rounded text-sm">Godkänn</button>
					<button onclick={rejectBooking} class="bg-red-600 text-white px-4 py-2 rounded text-sm">Neka</button>
				</div>
			</div>
		{/if}

		<div class="flex items-center gap-2">
			{#if editable}
				<a href="/book?id={booking.id}" class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Redigera</a>
			{/if}
			{#if booking.status === 'draft'}
				<div class="border rounded p-4 mb-3 bg-neutral-50">
					<textarea
						bind:value={submitMessage}
						placeholder="Meddelande till utrustningsansvarig (valfritt)"
						class="w-full border rounded px-3 py-2 text-sm mb-2"
						rows="2"
					></textarea>
					<div class="flex items-center gap-3">
						<button onclick={submitBooking} class="bg-green-700 text-white px-4 py-2 rounded text-sm">Skicka bokning</button>
						<label class="flex items-center gap-1.5 text-sm text-neutral-600">
							<input type="checkbox" bind:checked={forceApproval} />
							Vill ha bekräftelse från ansvarig
						</label>
					</div>
				</div>
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
		{#if showReturn}
			<ReturnChecklist
				bookingId={booking.id}
				{items}
				onUpdate={handleReturnUpdate}
				onBookingReturned={handleBookingReturned}
			/>
		{:else}
			<PickupChecklist
				bookingId={booking.id}
				{items}
				startDate={booking.start_date}
				endDate={booking.end_date}
				onUpdate={handlePickupUpdate}
			/>
			{#if anyPickedUp}
				<button onclick={() => showReturn = true} class="mt-4 bg-blue-700 text-white px-4 py-2 rounded text-sm">Starta återlämning</button>
			{/if}
		{/if}
	{:else if booking.status === 'returned'}
		<BookingItemsList {items} />
		<button onclick={reopenBooking} class="mt-4 text-sm text-blue-700 underline">Öppna igen</button>
	{:else}
		<BookingItemsList {items} />
	{/if}
</div>
