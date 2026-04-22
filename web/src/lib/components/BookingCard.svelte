<script lang="ts">
	import type { Booking } from '$lib/api/client';
	import { msg } from '$lib/msg';
	import { bookingStatusColors, bookingStatusLeftBorder } from '$lib/styles';

	interface Props {
		booking: Booking;
		href: string;
	}

	let { booking, href }: Props = $props();
</script>

<a {href} class="block border rounded px-4 py-3 hover:bg-neutral-50 {bookingStatusLeftBorder[booking.status] ?? ''}">
	<div class="flex flex-wrap items-center justify-between gap-1">
		<div class="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
			<span class="font-medium">{booking.start_date} — {booking.end_date}</span>
			{#if booking.team_name}
				<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">{booking.team_name}</span>
			{:else if booking.used_by_external}
				<span class="text-xs bg-neutral-50 text-neutral-600 px-1.5 py-0.5 rounded">{booking.used_by_external}</span>
			{:else}
				<span class="text-xs text-neutral-400">Personlig</span>
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
