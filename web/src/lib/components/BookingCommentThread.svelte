<script lang="ts">
	import { createApiClient, type BookingEvent } from '$lib/api/client';
	import UserBadge from '$lib/components/UserBadge.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';

	interface Props {
		bookingId: string;
		// Bump this from the parent after an action (submit/approve/reject) that
		// logs an event outside this component, to trigger a reload.
		refreshKey?: number;
	}

	let { bookingId, refreshKey = 0 }: Props = $props();

	const api = createApiClient();

	let events = $state<BookingEvent[]>([]);
	let noteMessage = $state('');
	let error = $state('');

	async function loadEvents() {
		try {
			events = await api.listBookingEvents(bookingId);
		} catch { /* ignore */ }
	}

	$effect(() => {
		refreshKey;
		loadEvents();
	});

	async function addNote() {
		if (!noteMessage.trim()) return;
		error = '';
		try {
			await api.addBookingNote(bookingId, noteMessage);
			noteMessage = '';
			loadEvents();
		} catch (e) {
			error = translateError(e);
		}
	}
</script>

<h2 class="font-medium mb-2">{m.page_booking_comment_thread_heading()}</h2>

{#if error}
	<p class="text-red-600 text-xs mb-2">{error}</p>
{/if}

{#if events.length > 0}
	<div class="border rounded mb-3 divide-y">
		{#each events as event}
			<div class="px-4 py-2 text-sm {event.event_type === 'rejected' ? 'bg-red-50' : event.event_type === 'approved' ? 'bg-green-50' : 'bg-neutral-50'}">
				<div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-neutral-500 mb-0.5">
					<UserBadge userId={event.actor_id} name={event.actor_name} picture={event.actor_picture} contextBookingId={bookingId} size={18} />
					{#if event.event_type !== 'items_changed'}
						<span>
							{({'submitted': m.page_booking_event_submitted(), 'approved': m.page_booking_event_approved(), 'rejected': m.page_booking_event_rejected(), 'cancelled': m.page_booking_event_cancelled(), 'note': m.page_booking_event_commented()} as Record<string,string>)[event.event_type] ?? event.event_type}
						</span>
					{/if}
					<span>{new Date(event.created_at).toLocaleDateString('sv', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span>
				</div>
				{#if event.message}
					<p class="text-neutral-700">{event.message}</p>
				{/if}
			</div>
		{/each}
	</div>
{/if}

<div class="flex gap-2 mb-4">
	<input
		type="text"
		bind:value={noteMessage}
		placeholder={m.page_booking_message_to_manager()}
		class="flex-1 border rounded px-3 py-2 text-sm"
		onkeydown={(e) => { if (e.key === 'Enter') addNote(); }}
	/>
	<button onclick={addNote} disabled={!noteMessage.trim()} class="bg-neutral-700 text-white px-3 py-2 rounded text-sm disabled:opacity-50">{m.page_booking_btn_send_comment()}</button>
</div>
