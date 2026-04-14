<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { cart } from '$lib/stores/cart.svelte';
	import BookingItemsList from '$lib/components/BookingItemsList.svelte';
	import type { PageData } from './$types';

	const accessLabels: Record<string, string> = {
		view: 'Visa',
		book: 'Boka',
		trusted: 'Betrodd',
		manager: 'Ansvarig'
	};

	let { data }: { data: PageData } = $props();

	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let myTeamSet = $derived(new Set(($page.data.user?.teams ?? []).map((t: { team_name: string }) => t.team_name)));
	let userTeams = $derived.by(() => {
		const all = isManager ? data.teams : data.teams.filter(u => myTeamSet.has(u.name));
		return [...all].sort((a, b) => {
			const aIsMine = myTeamSet.has(a.name) ? 0 : 2;
			const bIsMine = myTeamSet.has(b.name) ? 0 : 2;
			const aType = a.type === 'troop' ? 0 : 1;
			const bType = b.type === 'troop' ? 0 : 1;
			const aKey = aIsMine + aType;
			const bKey = bIsMine + bType;
			if (aKey !== bKey) return aKey - bKey;
			return a.name.localeCompare(b.name);
		});
	});

	// Create mode state
	let newStartDate = $state('');
	let newEndDate = $state('');
	let newNotes = $state('');
	let defaultUnit = $derived.by(() => {
		const myTeams = data.teams.filter(u => myTeamSet.has(u.name));
		const troop = myTeams.find(u => u.type === 'troop');
		if (troop) return troop.id;
		const role = myTeams.find(u => u.type === 'role');
		if (role) return role.id;
		return '';
	});
	let newUnit = $state('');
	let newUnitInitialized = $state(false);
	$effect(() => {
		if (!newUnitInitialized && defaultUnit) {
			newUnit = defaultUnit;
			newUnitInitialized = true;
		}
	});
	let creating = $state(false);
	let createError = $state('');

	async function createBooking() {
		if (!newStartDate || !newEndDate) return;
		creating = true;
		createError = '';
		try {
			const booking = await api.createBooking({
				start_date: newStartDate,
				end_date: newEndDate,
				notes: newNotes,
				used_by_team_id: newUnit || undefined
			});
			cart.activate(booking.id);
			goto(`/book?id=${booking.id}`);
		} catch (e: any) {
			createError = e.message;
		}
		creating = false;
	}

	// Cart management state (when ?id is present)
	let bookingId = $derived(data.existing?.booking.id ?? null);
	let startDate = $state('');
	let endDate = $state('');
	let notes = $state('');
	let selectedUnit = $state('');
	let unitInitialized = $state(false);
	let cartItems = $state<BookingItem[]>([]);
	let error = $state('');
	let message = $state('');
	let saving = $state(false);
	let submitting = $state(false);
	let submitted = $state(false);

	// Conflict detection
	let conflictingIds = $state<Set<string>>(new Set());
	let hasConflicts = $derived(conflictingIds.size > 0);

	$effect(() => {
		if (data.existing) {
			startDate = data.existing.booking.start_date;
			endDate = data.existing.booking.end_date;
			notes = data.existing.booking.notes;
			selectedUnit = data.existing.booking.used_by_team_id ?? '';
			unitInitialized = true;
			cartItems = data.existing.items;
			// Activate cart if not already
			if (data.existing.booking.status === 'draft') {
				cart.activate(data.existing.booking.id);
			}
		}
	});

	$effect(() => {
		if (!unitInitialized && defaultUnit) {
			selectedUnit = defaultUnit;
			unitInitialized = true;
		}
	});

	function showMessage(msg: string) {
		message = msg;
		setTimeout(() => message = '', 4000);
	}

	async function saveDetails() {
		if (!bookingId) return;
		saving = true;
		error = '';
		conflictingIds = new Set();
		try {
			await api.updateBooking(bookingId, {
				start_date: startDate,
				end_date: endDate,
				notes,
				used_by_team_id: selectedUnit || ''
			});
			// Check for conflicts after date change
			const available = await api.listAvailableArticles(startDate, endDate, { exclude_booking_id: bookingId });
			const availableIds = new Set(available.map(a => a.id));
			const conflicts = new Set(
				cartItems
					.filter(item => !availableIds.has(item.article_id))
					.map(item => item.article_id)
			);
			conflictingIds = conflicts;
			if (conflicts.size === 0) {
				showMessage('Ändringar sparade');
			}
		} catch (e: any) {
			error = e.message;
		}
		saving = false;
	}

	async function addOneToCart(commercialName: string, locationName: string) {
		if (!bookingId) return;
		error = '';
		try {
			await api.addBookingItems(bookingId, commercialName, 1, locationName);
			const result = await api.getBooking(bookingId);
			cartItems = result.items;
			cart.refresh(); // Notify FloatingCart to reload
		} catch (e: any) {
			error = e.message;
		}
	}

	async function removeFromCart(itemId: string) {
		if (!bookingId) return;
		error = '';
		const articleId = cartItems.find(i => i.id === itemId)?.article_id;
		try {
			await api.removeBookingItem(bookingId, itemId);
			const result = await api.getBooking(bookingId);
			cartItems = result.items;
			if (articleId && conflictingIds.has(articleId)) {
				const next = new Set(conflictingIds);
				next.delete(articleId);
				conflictingIds = next;
			}
		} catch (e: any) {
			error = e.message;
		}
	}

	async function removeOneFromCart(commercialName: string, locationName: string) {
		if (!bookingId) return;
		const matching = cartItems.filter(i => i.commercial_name === commercialName && i.location_name === locationName);
		if (matching.length === 0) return;
		// Sort by status to remove low-priority items first
		const statusPriority: Record<string, number> = {
			'reported_usable': 0,
			'under_repair': 1,
			'incoming': 2,
			'ok': 3
		};
		matching.sort((a, b) => (statusPriority[a.article_status] ?? 4) - (statusPriority[b.article_status] ?? 4));
		await removeFromCart(matching[0].id);
		cart.refresh(); // Notify FloatingCart to reload
	}

	async function submitBooking() {
		if (!bookingId || hasConflicts) return;
		submitting = true;
		error = '';
		try {
			await api.updateBooking(bookingId, {
				start_date: startDate,
				end_date: endDate,
				notes,
				used_by_team_id: selectedUnit || ''
			});
			const booking = await api.submitBooking(bookingId);
			cart.clear();
			submitted = true;
			message = booking.status === 'confirmed' ? 'Bokning bekräftad!' : 'Bokning inskickad!';
		} catch (e: any) {
			error = e.message;
		}
		submitting = false;
	}

	async function cancelBooking() {
		if (!bookingId) return;
		if (!confirm('Är du säker?')) return;
		try {
			await api.cancelBooking(bookingId);
			cart.clear();
			window.location.href = '/';
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

<div class="max-w-4xl mx-auto p-4">
	{#if !data.existing}
		<!-- Create mode -->
		<h1 class="text-heading-sm font-bold mb-4">Ny bokning</h1>

		{#if createError}
			<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{createError}</div>
		{/if}

		<div class="grid grid-cols-2 sm:flex sm:flex-wrap gap-3 mb-4">
			<label class="flex flex-col gap-1">
				<span class="text-sm">Startdatum</span>
				<input type="date" bind:value={newStartDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">Slutdatum</span>
				<input type="date" bind:value={newEndDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">Anteckningar</span>
				<input type="text" bind:value={newNotes} placeholder="T.ex. Hajk med Yggdrasil" class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">Bokas för</span>
				<select bind:value={newUnit} class="border rounded px-3 py-2">
					{#each userTeams.filter(u => myTeamSet.has(u.name)) as unit}
						<option value={unit.id}>{unit.name} ({accessLabels[unit.access_level] ?? unit.access_level})</option>
					{/each}
					<option value="">Personlig bokning</option>
					{#if isManager}
						{@const otherTeams = userTeams.filter(u => !myTeamSet.has(u.name))}
						{#if otherTeams.length > 0}
							<option disabled>───</option>
							{#each otherTeams as unit}
								<option value={unit.id}>{unit.name} ({accessLabels[unit.access_level] ?? unit.access_level})</option>
							{/each}
						{/if}
					{/if}
				</select>
			</label>
		</div>

		<scout-button
			type="button"
			variant="primary"
			onclick={createBooking}
			disabled={!newStartDate || !newEndDate || creating ? true : undefined}
		>
			{creating ? '...' : 'Skapa bokning'}
		</scout-button>

	{:else if submitted}
		<div class="bg-green-50 border border-green-200 rounded p-4">
			<p class="font-medium text-green-800">{message}</p>
			<a href="/bookings/{bookingId}" class="underline text-green-700">Visa bokning →</a>
		</div>

	{:else}
		<!-- Cart management mode -->
		<h1 class="text-heading-sm font-bold mb-4">Din bokning</h1>

		{#if message}
			<div class="bg-green-50 border border-green-200 rounded p-3 mb-4 text-green-800 text-sm">{message}</div>
		{/if}

		{#if error}
			<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{error}</div>
		{/if}

		{#if hasConflicts}
			<div class="bg-orange-50 border border-orange-200 rounded p-3 mb-4 text-orange-800 text-sm">
				<p class="font-medium mb-1">Datumkonflikt - {conflictingIds.size} artiklar inte tillgängliga för de nya datumen.</p>
				<p>Ta bort de markerade artiklarna eller ändra tillbaka datumen för att kunna skicka bokningen.</p>
			</div>
		{/if}

		<!-- Dates, team, notes -->
		<div class="grid grid-cols-2 sm:flex sm:flex-wrap gap-3 mb-3">
			<label class="flex flex-col gap-1">
				<span class="text-sm">Startdatum</span>
				<input type="date" bind:value={startDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">Slutdatum</span>
				<input type="date" bind:value={endDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">Anteckningar</span>
				<input type="text" bind:value={notes} placeholder="T.ex. Hajk med Yggdrasil" class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">Bokas för</span>
				<select bind:value={selectedUnit} class="border rounded px-3 py-2">
					{#each userTeams.filter(u => myTeamSet.has(u.name)) as unit}
						<option value={unit.id}>{unit.name} ({accessLabels[unit.access_level] ?? unit.access_level})</option>
					{/each}
					<option value="">Personlig bokning</option>
					{#if isManager}
						{@const otherTeams = userTeams.filter(u => !myTeamSet.has(u.name))}
						{#if otherTeams.length > 0}
							<option disabled>───</option>
							{#each otherTeams as unit}
								<option value={unit.id}>{unit.name} ({accessLabels[unit.access_level] ?? unit.access_level})</option>
							{/each}
						{/if}
					{/if}
				</select>
			</label>
		</div>

		<div class="flex gap-2 mb-6">
			<scout-button
				type="button"
				variant="outlined"
				onclick={saveDetails}
				disabled={saving ? true : undefined}
			>
				{saving ? '...' : 'Spara ändringar'}
			</scout-button>
		</div>

		<!-- Item list -->
		{#if cartItems.length > 0}
			<h2 class="font-medium mb-2">Utrustning ({cartItems.length} artiklar)</h2>
			<BookingItemsList items={cartItems} editable conflictingIds={conflictingIds} onRemove={removeFromCart} onAddOne={addOneToCart} onRemoveOne={removeOneFromCart} />
		{:else}
			<p class="text-sm text-neutral-500 mb-4">Inga artiklar tillagda ännu.</p>
		{/if}

		<!-- Primary actions -->
		<div class="flex flex-wrap gap-3 mt-6">
			<scout-button type="link" href="/browse" variant="outlined">Lägg till utrustning</scout-button>
			<scout-button
				type="button"
				variant="primary"
				onclick={submitBooking}
				disabled={cartItems.length === 0 || hasConflicts || submitting ? true : undefined}
			>
				{submitting ? '...' : 'Skicka bokning'}
			</scout-button>
		</div>

		<!-- Cancel - separated so it's not accidentally tapped -->
		<div class="mt-6 pt-4 border-t">
			<scout-button type="button" variant="danger" size="large" onclick={cancelBooking}>
				Avbryt bokning
			</scout-button>
		</div>
	{/if}
</div>
