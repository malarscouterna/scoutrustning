<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	let isManager = $derived(data.user?.roles.includes('equipment_manager') ?? false);
	let userUnits = $derived(data.user?.units ?? []);

	const initialFilter: 'mine' | 'all' | 'pending' =
		data.user?.roles.includes('equipment_manager')
			? (data.pendingCount > 0 ? 'pending' : 'all')
			: 'mine';
	let filter = $state<'mine' | 'all' | 'pending'>(initialFilter);

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

	function isMine(booking: any): boolean {
		if (booking.created_by === data.user?.member_id) return true;
		if (booking.unit_name && userUnits.includes(booking.unit_name)) return true;
		return false;
	}

	let filteredBookings = $derived.by(() => {
		if (!isManager) return data.bookings;
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
		<h1 class="text-heading-sm font-bold">Bokningar</h1>
		<a href="/book" class="bg-blue-700 text-white px-4 py-2 rounded text-sm">Ny bokning</a>
	</div>

	{#if isManager}
		<div class="flex gap-2 mb-4">
			{#if data.pendingCount > 0}
				<button
					onclick={() => filter = 'pending'}
					class="px-3 py-1.5 rounded text-sm flex items-center gap-1.5 {filter === 'pending' ? 'bg-orange-600 text-white' : 'bg-orange-50 text-orange-700'}"
				>
					Väntar på godkännande
					<span class="text-xs px-1.5 py-0.5 rounded-full {filter === 'pending' ? 'bg-white/20' : 'bg-orange-200'}">{data.pendingCount}</span>
				</button>
			{/if}
			<button
				onclick={() => filter = 'mine'}
				class="px-3 py-1.5 rounded text-sm {filter === 'mine' ? 'bg-blue-700 text-white' : 'bg-neutral-100'}"
			>Mina</button>
			<button
				onclick={() => filter = 'all'}
				class="px-3 py-1.5 rounded text-sm {filter === 'all' ? 'bg-blue-700 text-white' : 'bg-neutral-100'}"
			>Alla</button>
		</div>
	{/if}

	{#if filteredBookings.length === 0}
		<p class="text-neutral-500">
			{#if filter === 'pending'}
				Inga bokningar väntar på godkännande.
			{:else if filter === 'mine'}
				Du har inga bokningar ännu.
			{:else}
				Inga bokningar ännu.
			{/if}
		</p>
	{:else}
		<div class="space-y-2">
			{#each filteredBookings as booking}
				<a href="/bookings/{booking.id}" class="block border rounded px-4 py-3 hover:bg-neutral-50">
					<div class="flex flex-wrap items-center justify-between gap-1">
						<div class="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
							<span class="font-medium">{booking.start_date} — {booking.end_date}</span>
							{#if booking.unit_name}
								<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">{booking.unit_name}</span>
							{:else if booking.used_by_external}
								<span class="text-xs bg-neutral-50 text-neutral-600 px-1.5 py-0.5 rounded">{booking.used_by_external}</span>
							{:else}
								<span class="text-xs text-neutral-400">Personlig</span>
							{/if}
						</div>
						<span class="text-xs px-2 py-0.5 rounded shrink-0 {statusColors[booking.status] ?? 'bg-neutral-100'}">
							{statusLabels[booking.status] ?? booking.status}
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
