<script lang="ts">
	import type { Booking } from '$lib/api/client';

	interface Props {
		booking: Booking;
		href: string;
	}

	let { booking, href }: Props = $props();

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
		approved: 'bg-blue-100 text-blue-800',
		confirmed: 'bg-green-100 text-green-800',
		picked_up: 'bg-blue-100 text-blue-800',
		returned: 'bg-neutral-100',
		rejected: 'bg-red-100 text-red-800',
		cancelled: 'bg-neutral-100 text-neutral-500'
	};

	const leftBorder: Record<string, string> = {
		submitted: 'border-l-4 border-l-orange-400',
		approved: 'border-l-4 border-l-blue-500',
		confirmed: 'border-l-4 border-l-blue-500',
		picked_up: 'border-l-4 border-l-blue-500',
	};
</script>

<a {href} class="block border rounded px-4 py-3 hover:bg-neutral-50 {leftBorder[booking.status] ?? ''}">
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
		<span class="text-xs px-2 py-0.5 rounded {statusColors[booking.status] ?? 'bg-neutral-100'}">
			{statusLabels[booking.status] ?? booking.status}
		</span>
	</div>
	{#if booking.notes}
		<p class="text-sm text-neutral-500 mt-1 truncate">{booking.notes}</p>
	{/if}
</a>
