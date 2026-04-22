<script lang="ts">
	import { isManager as checkManager } from '$lib/user';
	import type { PageData } from './$types';
	import { msg } from '$lib/msg';
	import { bookingStatusColors } from '$lib/styles';
	import * as m from '$lib/paraglide/messages.js';

	let { data }: { data: PageData } = $props();

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
				<a href="/bookings/{booking.id}" class="block border rounded px-4 py-3 hover:bg-neutral-50">
					<div class="flex flex-wrap items-center justify-between gap-1">
						<div class="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
							<span class="font-medium">{booking.start_date} — {booking.end_date}</span>
							{#if booking.team_name}
								<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">{booking.team_name}</span>
							{:else if booking.used_by_external}
								<span class="text-xs bg-neutral-50 text-neutral-600 px-1.5 py-0.5 rounded">{booking.used_by_external}</span>
							{:else}
								<span class="text-xs text-neutral-400">{m.page_bookings_personal()}</span>
							{/if}
						</div>
						<span class="text-xs px-2 py-0.5 rounded {bookingStatusColors[booking.status] ?? 'bg-neutral-100'}">
							{msg(`booking_status_${booking.status}`) ?? booking.status}
						</span>
					</div>
					{#if booking.notes}
						<p class="text-sm text-neutral-500 mt-1 truncate">{booking.notes}</p>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</div>
