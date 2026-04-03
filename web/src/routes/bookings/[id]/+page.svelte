<script lang="ts">
	import { createApiClient, type Booking, type BookingItem } from '$lib/api/client';
	import type { PageData } from './$types';
	import { goto } from '$app/navigation';

	let { data }: { data: PageData } = $props();

	const api = createApiClient({ persona: 'leader-yggdrasil' });

	let booking = $state<Booking>(data.booking);
	let items = $state<BookingItem[]>(data.items);
	let editing = $state(false);
	let editNotes = $state(booking.notes);
	let editStartDate = $state(booking.start_date);
	let editEndDate = $state(booking.end_date);
	let editUnitId = $state(booking.used_by_unit_id ?? '');
	let error = $state('');
	let saving = $state(false);

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

	// Group items by commercial_name
	let itemGroups = $derived.by(() => {
		const map = new Map<string, { commercialName: string; items: BookingItem[] }>();
		for (const item of items) {
			const existing = map.get(item.commercial_name);
			if (existing) {
				existing.items.push(item);
			} else {
				map.set(item.commercial_name, { commercialName: item.commercial_name, items: [item] });
			}
		}
		return [...map.values()];
	});

	async function saveEdit() {
		error = '';
		saving = true;
		try {
			const updated: Record<string, unknown> = {};
			if (editNotes !== booking.notes) updated.notes = editNotes;
			if (editStartDate !== booking.start_date) updated.start_date = editStartDate;
			if (editEndDate !== booking.end_date) updated.end_date = editEndDate;
			const newUnitId = editUnitId || null;
			if (newUnitId !== (booking.used_by_unit_id ?? null)) updated.used_by_unit_id = newUnitId;

			if (Object.keys(updated).length > 0) {
				booking = await api.updateBooking(booking.id, updated);
			}
			editing = false;
		} catch (e: any) {
			error = e.message;
		}
		saving = false;
	}

	async function removeItem(itemId: string) {
		error = '';
		try {
			await api.removeBookingItem(booking.id, itemId);
			const result = await api.getBooking(booking.id);
			items = result.items;
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
				goto('/bookings');
			} else {
				booking = { ...booking, status: 'cancelled' };
			}
		} catch (e: any) {
			error = e.message;
		}
	}

	async function copyBooking() {
		error = '';
		try {
			const result = await api.copyBooking(booking.id);
			goto('/bookings/' + result.booking.id);
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

<div class="max-w-4xl mx-auto p-4">
	<a href="/bookings" class="text-sm text-blue-700 underline">← Tillbaka</a>

	{#if error}
		<div class="bg-red-50 border border-red-200 rounded p-3 mt-4 text-red-800 text-sm">{error}</div>
	{/if}

	<div class="mt-4 mb-6">
		{#if editing}
			<div class="space-y-3 border rounded p-4 bg-neutral-50">
				<div class="flex flex-wrap gap-3">
					<label class="flex flex-col gap-1">
						<span class="text-sm">Startdatum</span>
						<input type="date" bind:value={editStartDate} class="border rounded px-3 py-2" />
					</label>
					<label class="flex flex-col gap-1">
						<span class="text-sm">Slutdatum</span>
						<input type="date" bind:value={editEndDate} class="border rounded px-3 py-2" />
					</label>
				</div>
				<label class="flex flex-col gap-1">
					<span class="text-sm">Anteckningar</span>
					<input type="text" bind:value={editNotes} class="border rounded px-3 py-2" />
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-sm">Bokas för</span>
					<select bind:value={editUnitId} class="border rounded px-3 py-2">
						<option value="">Personlig bokning</option>
						{#each data.units as unit}
							<option value={unit.id}>{unit.name}</option>
						{/each}
					</select>
				</label>
				<div class="flex gap-2">
					<button onclick={saveEdit} disabled={saving} class="bg-blue-700 text-white px-4 py-2 rounded text-sm disabled:opacity-50">
						{saving ? 'Sparar...' : 'Spara'}
					</button>
					<button onclick={() => { editing = false; error = ''; }} class="border rounded px-4 py-2 text-sm">Avbryt</button>
				</div>
			</div>
		{:else}
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
			<p class="text-sm text-neutral-500 mb-2">
				{#if booking.unit_name}
					För: <span class="font-medium text-blue-700">{booking.unit_name}</span>
				{:else if booking.used_by_external}
					För: {booking.used_by_external}
				{:else}
					Personlig bokning
				{/if}
			</p>
			<div class="flex gap-2">
				{#if editable}
					<button onclick={() => editing = true} class="text-sm text-blue-700 underline">Redigera</button>
				{/if}
				<button onclick={copyBooking} class="text-sm text-blue-700 underline">Kopiera</button>
				{#if cancellable}
					<button onclick={cancelBooking} class="text-sm text-red-600 underline">
						{booking.status === 'draft' ? 'Ta bort utkast' : 'Avboka'}
					</button>
				{/if}
			</div>
		{/if}
	</div>

	<h2 class="font-medium mb-2">Utrustning ({items.length} artiklar)</h2>

	{#if items.length === 0}
		<p class="text-neutral-500">Inga artiklar i bokningen.</p>
	{:else}
		<div class="space-y-2">
			{#each itemGroups as group}
				<div class="border rounded">
					<div class="px-4 py-2 font-medium bg-neutral-50 border-b">
						{group.commercialName} × {group.items.length}
					</div>
					<table class="w-full text-sm">
						<tbody>
							{#each group.items as item}
								<tr class="border-t first:border-t-0">
									<td class="px-4 py-2">{item.common_name}</td>
									<td class="px-4 py-2 text-neutral-600">{item.location_name}</td>
									<td class="px-4 py-2 text-neutral-600">{item.place || ''}</td>
									{#if editable}
										<td class="px-4 py-2 text-right">
											<button onclick={() => removeItem(item.id)} class="text-red-600 text-xs hover:underline">Ta bort</button>
										</td>
									{/if}
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/each}
		</div>
	{/if}
</div>
