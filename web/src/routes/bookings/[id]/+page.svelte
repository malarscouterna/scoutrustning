<script lang="ts">
	import { createApiClient, type Booking, type BookingItem, type BookingEvent } from '$lib/api/client';
	import BookingItemsList from '$lib/components/BookingItemsList.svelte';
	import PickupChecklist from '$lib/components/PickupChecklist.svelte';
	import ReturnChecklist from '$lib/components/ReturnChecklist.svelte';
	import AddItemSheet from '$lib/components/AddItemSheet.svelte';
	import { isManager as checkManager } from '$lib/user';
	import { cart } from '$lib/stores/cart.svelte';
	import type { PageData } from './$types';
	import { page } from '$app/stores';
	import { onDestroy } from 'svelte';
	import { msg } from '$lib/msg';
	import { bookingStatusColors } from '$lib/styles';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';

	let { data }: { data: PageData } = $props();

	const api = createApiClient();

	// svelte-ignore state_referenced_locally
	let booking = $state(data.booking);
	// svelte-ignore state_referenced_locally
	let items = $state(data.items);
	let error = $state('');
	let message = $state('');

	$effect(() => {
		booking = data.booking;
		items = data.items;
	});

	$effect(() => {
		const urlMsg = $page.url.searchParams.get('msg');
		if (urlMsg) {
			message = urlMsg;
			history.replaceState({}, '', $page.url.pathname);
			setTimeout(() => message = '', 4000);
		}
	});

	let reloading = false;

	async function reload(): Promise<BookingItem[]> {
		reloading = true;
		try {
			const result = await api.getBooking(booking.id);
			items = result.items;
			return result.items;
		} finally {
			reloading = false;
		}
	}

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	function startPolling() {
		stopPolling();
		pollTimer = setInterval(async () => {
			if (reloading) return;
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
		if (active) startPolling(); else stopPolling();
	});

	onDestroy(stopPolling);

	let editable = $derived(
		['draft', 'submitted', 'approved', 'confirmed'].includes(booking.status)
	);

	let cancellable = $derived(
		booking.status !== 'returned' && booking.status !== 'cancelled'
	);

	let anyPickedUp = $derived(items.some((i) => i.pickup_status));
	let allPickedUp = $derived(items.length > 0 && items.every((i) => i.pickup_status));
	let anyReturnStarted = $derived(items.some((i) => i.return_status && i.return_status !== 'pending'));

	let pickupMode = $state(false);
	let returnMode = $state(false);
	let showAddItemSheet = $state(false);

	// Auto-resume return mode if a return was already started
	$effect(() => {
		if (anyReturnStarted && booking.status === 'picked_up') {
			returnMode = true;
		}
	});

	async function submitBooking() {
		error = '';
		try {
			booking = await api.submitBooking(booking.id, submitMessage || undefined, forceApproval || undefined);
			message = booking.status === 'confirmed' ? m.page_booking_confirmed() : m.page_booking_submitted();
			submitMessage = '';
			forceApproval = false;
			setTimeout(() => message = '', 4000);
			loadEvents();
		} catch (e) {
			error = translateError(e);
		}
	}

	async function cancelBooking() {
		if (!confirm(m.common_confirm())) return;
		error = '';
		try {
			await api.cancelBooking(booking.id);
			if (booking.status === 'draft') {
				window.location.href = '/bookings';
			} else {
				booking = { ...booking, status: 'cancelled' };
			}
		} catch (e) {
			error = translateError(e);
		}
	}

	async function startPickup() {
		error = '';
		const cartId = cart.id;
		if (cartId && cartId !== booking.id) {
			if (!confirm(m.page_booking_confirm_clear_cart())) return;
			cart.clear();
		}
		try {
			booking = await api.pickupBooking(booking.id);
			pickupMode = true;
			message = m.page_booking_pickup_started();
			setTimeout(() => message = '', 4000);
		} catch (e) {
			error = translateError(e);
		}
	}

	function handleBookingReturned() {
		booking = { ...booking, status: 'returned' };
		returnMode = false;
		message = m.page_booking_all_returned();
		setTimeout(() => message = '', 4000);
	}

	async function reopenBooking() {
		error = '';
		try {
			booking = { ...booking, status: 'picked_up' };
			returnMode = true;
			message = m.page_booking_reopened();
			setTimeout(() => message = '', 4000);
		} catch (e) {
			error = translateError(e);
		}
	}

	let approvalMessage = $state('');
	let submitMessage = $state('');
	let forceApproval = $state(false);
	let isManager = $derived(checkManager(data.user));
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
		} catch (e) {
			error = translateError(e);
		}
	}

	async function approveBooking() {
		error = '';
		try {
			booking = await api.approveBooking(booking.id, approvalMessage);
			message = m.page_booking_confirmed();
			approvalMessage = '';
			setTimeout(() => message = '', 4000);
			loadEvents();
		} catch (e) {
			error = translateError(e);
		}
	}

	async function rejectBooking() {
		error = '';
		try {
			booking = await api.rejectBooking(booking.id, approvalMessage);
			message = m.page_booking_rejected();
			approvalMessage = '';
			setTimeout(() => message = '', 4000);
			loadEvents();
		} catch (e) {
			error = translateError(e);
		}
	}
</script>

<div class="max-w-4xl mx-auto p-4" class:pb-24={pickupMode || returnMode}>
	{#if message}
		<div class="bg-green-50 border border-green-200 rounded p-3 mt-4 text-green-800 text-sm">{message}</div>
	{/if}

	{#if error}
		<div class="bg-red-50 border border-red-200 rounded p-3 mt-4 text-red-800 text-sm">{error}</div>
	{/if}

	{#if !pickupMode && !returnMode}
		<div class="mt-4 mb-6">
			<div class="flex items-center gap-3 mb-2">
				<h1 class="text-heading-sm font-bold">
					{booking.start_date} — {booking.end_date}
				</h1>
				<span class="text-sm px-2 py-0.5 rounded {bookingStatusColors[booking.status] ?? 'bg-neutral-100'}">
					{msg(`booking_status_${booking.status}`) ?? booking.status}
				</span>
			</div>
			{#if booking.notes}
				<p class="text-neutral-600 mb-2">{booking.notes}</p>
			{/if}
			<p class="text-sm text-neutral-500 mb-3">
				{#if booking.team_name}
					{m.page_booking_for_label()} <span class="font-medium text-blue-700">{booking.team_name}</span>
				{:else if booking.used_by_external}
					{m.page_booking_for_label()} {booking.used_by_external}
				{/if}
			</p>

			<!-- Action buttons up top -->
			{#if booking.status === 'picked_up'}
				<div class="flex flex-wrap gap-2 mb-4">
					<button onclick={() => pickupMode = true} class="bg-blue-700 text-white px-4 py-2 rounded text-sm">
						{allPickedUp ? m.page_booking_btn_view_pickup() : m.page_booking_btn_continue_pickup()}
					</button>
					<button onclick={() => returnMode = true} class="bg-green-700 text-white px-4 py-2 rounded text-sm">
						{anyReturnStarted ? m.page_booking_btn_continue_return() : m.page_booking_btn_start_return()}
					</button>
				</div>
			{/if}

			{#if bookingEvents.length > 0}
				<div class="border rounded mb-3 divide-y">
					{#each bookingEvents as event}
						<div class="px-4 py-2 text-sm {event.event_type === 'rejected' ? 'bg-red-50' : event.event_type === 'approved' ? 'bg-green-50' : 'bg-neutral-50'}">
							<div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-neutral-500 mb-0.5">
								<span class="font-medium text-neutral-700">{event.actor_name}</span>
								<span>
									{({'submitted': m.page_booking_event_submitted(), 'approved': m.page_booking_event_approved(), 'rejected': m.page_booking_event_rejected(), 'cancelled': m.page_booking_event_cancelled(), 'note': m.page_booking_event_commented()} as Record<string,string>)[event.event_type] ?? event.event_type}
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
					placeholder={m.page_booking_message_to_manager()}
					class="flex-1 border rounded px-3 py-2 text-sm"
					onkeydown={(e) => { if (e.key === 'Enter') addNote(); }}
				/>
				<button onclick={addNote} disabled={!noteMessage.trim()} class="bg-neutral-700 text-white px-3 py-2 rounded text-sm disabled:opacity-50">Skicka</button>
			</div>

			{#if isManager && booking.status === 'submitted'}
				<div class="border rounded p-4 mb-3 bg-orange-50">
					<p class="text-sm font-medium text-orange-800 mb-2">{m.page_booking_pending_message()}</p>
					<textarea
						bind:value={approvalMessage}
						placeholder={m.page_booking_message_to_booker()}
						class="w-full border rounded px-3 py-2 text-sm mb-2"
						rows="2"
					></textarea>
					<div class="flex gap-2">
						<button onclick={approveBooking} class="bg-green-700 text-white px-4 py-2 rounded text-sm">{m.page_booking_btn_approve()}</button>
						<button onclick={rejectBooking} class="bg-red-600 text-white px-4 py-2 rounded text-sm">{m.page_booking_btn_reject()}</button>
					</div>
				</div>
			{/if}

			<div class="flex flex-wrap items-center gap-2">
				{#if editable}
					<a href="/book?id={booking.id}" class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Redigera</a>
				{/if}
				{#if booking.status === 'draft'}
					<div class="border rounded p-4 mb-3 bg-neutral-50">
						<textarea
							bind:value={submitMessage}
							placeholder={m.page_booking_message_to_manager()}
							class="w-full border rounded px-3 py-2 text-sm mb-2"
							rows="2"
						></textarea>
						<div class="flex items-center gap-3">
							<button onclick={submitBooking} class="bg-green-700 text-white px-4 py-2 rounded text-sm">{m.page_booking_btn_submit()}</button>
							<label class="flex items-center gap-1.5 text-sm text-neutral-600">
								<input type="checkbox" bind:checked={forceApproval} />
								{m.page_booking_wants_confirmation()}
							</label>
						</div>
					</div>
				{/if}
				{#if booking.status === 'confirmed' || booking.status === 'approved'}
					<button onclick={startPickup} class="bg-blue-700 text-white px-4 py-2 rounded text-sm">{m.page_booking_btn_start_pickup()}</button>
				{/if}
				{#if cancellable}
					<button onclick={cancelBooking} class="text-sm text-red-600 underline">
						{booking.status === 'draft' ? m.page_booking_btn_delete_draft() : m.page_booking_btn_cancel()}
					</button>
				{/if}
			</div>
		</div>

		<h2 class="font-medium mb-2">{m.page_booking_items_heading({ count: String(items.length) })}</h2>
		{#if booking.status === 'returned'}
			<BookingItemsList {items} />
			<button onclick={reopenBooking} class="mt-4 text-sm text-blue-700 underline">{m.page_booking_btn_reopen()}</button>
		{:else}
			<BookingItemsList {items} />
		{/if}
	{:else if pickupMode}
		<PickupChecklist
			bookingId={booking.id}
			{items}
			startDate={booking.start_date}
			endDate={booking.end_date}
			onUpdate={reload}
		/>
	{:else if returnMode}
		<ReturnChecklist
			bookingId={booking.id}
			{items}
			onUpdate={reload}
			onBookingReturned={handleBookingReturned}
		/>
	{/if}
</div>

{#if pickupMode || returnMode}
	<div class="fixed bottom-0 left-0 right-0 bg-white border-t px-4 py-3 flex gap-3 z-40 shadow-lg">
		<button
			onclick={() => { pickupMode = false; returnMode = false; }}
			class="border rounded px-4 py-2 text-sm text-neutral-700"
		>← Tillbaka</button>
		{#if pickupMode}
			<button
				onclick={() => showAddItemSheet = true}
				class="bg-blue-700 text-white rounded px-4 py-2 text-sm"
			>+ {m.page_booking_btn_add_items()}</button>
		{/if}
	</div>
{/if}

{#if showAddItemSheet}
	<AddItemSheet
		bookingId={booking.id}
		startDate={booking.start_date}
		endDate={booking.end_date}
		onAdded={async () => { await reload(); showAddItemSheet = false; }}
		onClose={() => showAddItemSheet = false}
	/>
{/if}
