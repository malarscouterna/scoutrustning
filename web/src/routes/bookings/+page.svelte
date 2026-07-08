<script lang="ts">
	import { isManager as checkManager } from '$lib/user';
	import type { PageData } from './$types';
	import BookingCard from '$lib/components/BookingCard.svelte';
	import CopyBookingModal from '$lib/components/CopyBookingModal.svelte';
	import type { Booking } from '$lib/api/client';
	import { goto } from '$app/navigation';
	import * as m from '$lib/paraglide/messages.js';

	let { data }: { data: PageData } = $props();

	let copySource = $state<Booking | null>(null);
	function handleCopied(newBookingId: string) {
		copySource = null;
		goto(`/bookings/${newBookingId}?msg=${encodeURIComponent(m.copy_booking_success())}`);
	}

	let mgr = $derived(checkManager(data.user));
	let userTeamNames = $derived((data.user?.teams ?? []).map(t => t.team_name));

	let filter = $state<'mine' | 'all' | 'pending'>('mine');

	$effect(() => {
		filter = mgr
			? (data.pendingCount > 0 ? 'pending' : 'all')
			: 'mine';
	});

	function isMine(booking: any): boolean {
		if (booking.created_by === data.user?.member_id) return true;
		if (booking.team_name && userTeamNames.includes(booking.team_name)) return true;
		return false;
	}

	let filteredBookings = $derived.by(() => {
		if (!mgr) return data.bookings;
		switch (filter) {
			case 'pending':
				return data.bookings.filter(b => b.status === 'submitted');
			case 'mine':
				return data.bookings.filter(isMine);
			default:
				return data.bookings;
		}
	});
</script>

<div class="max-w-4xl mx-auto p-4">
	<div class="flex items-center justify-between mb-4">
		<h1 class="text-heading-sm font-bold">{m.page_bookings_heading()}</h1>
		<a href="/book" class="bg-blue-700 text-white px-4 py-2 rounded text-sm">{m.page_bookings_btn_new()}</a>
	</div>

	{#if mgr}
		<div class="flex gap-2 mb-4">
			{#if data.pendingCount > 0}
				<button
					onclick={() => filter = 'pending'}
					class="px-3 py-1.5 rounded text-sm flex items-center gap-1.5 {filter === 'pending' ? 'bg-orange-600 text-white' : 'bg-orange-50 text-orange-700'}"
				>
					{m.page_bookings_filter_pending()}
					<span class="text-xs px-1.5 py-0.5 rounded-full {filter === 'pending' ? 'bg-white/20' : 'bg-orange-200'}">{data.pendingCount}</span>
				</button>
			{/if}
			<button
				onclick={() => filter = 'mine'}
				class="px-3 py-1.5 rounded text-sm {filter === 'mine' ? 'bg-blue-700 text-white' : 'bg-neutral-100'}"
			>{m.page_bookings_filter_mine()}</button>
			<button
				onclick={() => filter = 'all'}
				class="px-3 py-1.5 rounded text-sm {filter === 'all' ? 'bg-blue-700 text-white' : 'bg-neutral-100'}"
			>{m.page_bookings_filter_all()}</button>
		</div>
	{/if}

	{#if filteredBookings.length === 0}
		<p class="text-neutral-500">
			{#if filter === 'pending'}
				{m.page_bookings_empty_pending()}
			{:else if filter === 'mine'}
				{m.page_bookings_empty_mine()}
			{:else}
				{m.page_bookings_empty_all()}
			{/if}
		</p>
	{:else}
		<div class="space-y-2">
			{#each filteredBookings as booking}
				<BookingCard {booking} href="/bookings/{booking.id}" onCopy={() => (copySource = booking)} />
			{/each}
		</div>
	{/if}
</div>

{#if copySource}
	<CopyBookingModal source={copySource} onClose={() => (copySource = null)} onCopied={handleCopied} />
{/if}
