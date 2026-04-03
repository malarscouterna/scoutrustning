<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

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
		approved: 'bg-blue-100 text-blue-800',
		confirmed: 'bg-green-100 text-green-800',
		picked_up: 'bg-blue-100 text-blue-800',
		returned: 'bg-neutral-100',
		rejected: 'bg-red-100 text-red-800',
		cancelled: 'bg-neutral-100 text-neutral-500'
	};
</script>

<div class="max-w-4xl mx-auto p-4">
	<div class="flex items-center justify-between mb-4">
		<h1 class="text-heading-sm font-bold">Mina bokningar</h1>
		<a href="/book" class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Ny bokning</a>
	</div>

	{#if data.bookings.length === 0}
		<p class="text-neutral-500">Inga bokningar ännu.</p>
	{:else}
		<div class="space-y-2">
			{#each data.bookings as booking}
				<a href="/bookings/{booking.id}" class="block border rounded px-4 py-3 hover:bg-neutral-50">
					<div class="flex items-center justify-between">
						<div>
							<span class="font-medium">{booking.start_date} — {booking.end_date}</span>
							{#if booking.unit_name}
								<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded ml-2">{booking.unit_name}</span>
							{:else if booking.used_by_external}
								<span class="text-xs bg-neutral-50 text-neutral-600 px-1.5 py-0.5 rounded ml-2">{booking.used_by_external}</span>
							{:else}
								<span class="text-xs text-neutral-400 ml-2">Personlig</span>
							{/if}
							{#if booking.notes}
								<span class="text-sm text-neutral-500 ml-2">{booking.notes}</span>
							{/if}
						</div>
						<span class="text-xs px-2 py-0.5 rounded {statusColors[booking.status] ?? 'bg-neutral-100'}">
							{statusLabels[booking.status] ?? booking.status}
						</span>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
