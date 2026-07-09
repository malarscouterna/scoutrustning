<script lang="ts">
	import { createApiClient, type BookingItem, ApiError } from '$lib/api/client';
	import { hasRole, canBookPersonal } from '$lib/user';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { cart } from '$lib/stores/cart.svelte';
	import BookingItemsList from '$lib/components/BookingItemsList.svelte';
	import BookingCommentThread from '$lib/components/BookingCommentThread.svelte';
	import type { PageData } from './$types';
	import { msg } from '$lib/msg';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';

	let { data }: { data: PageData } = $props();

	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let canPersonal = $derived(canBookPersonal($page.data.user));
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
	let newTitle = $state('');
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
		if (!newStartDate) {
			createError = m.error_invalid_start_date();
			return;
		}
		if (!newEndDate) {
			createError = m.error_invalid_end_date();
			return;
		}
		if (!newTitle.trim()) {
			createError = m.error_title_is_required();
			return;
		}
		creating = true;
		createError = '';
		try {
			const booking = await api.createBooking({
				start_date: newStartDate,
				end_date: newEndDate,
				title: newTitle,
				used_by_team_id: newUnit || undefined
			});
			cart.activate(booking.id);
			goto(`/book?id=${booking.id}`);
		} catch (e) {
			createError = translateError(e);
		}
		creating = false;
	}

	// Cart management state (when ?id is present)
	let bookingId = $derived(data.existing?.booking.id ?? null);
	let startDate = $state('');
	let endDate = $state('');
	let title = $state('');
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

	let cancellable = $derived(
		['draft', 'submitted', 'approved', 'confirmed', 'rejected'].includes(data.existing?.booking.status ?? '')
	);

	// Auto-archive countdown (docs/implementation/pre-release.md "Booking auto-archive setting") - shown
	// here too, not just the read-only detail page, since this is where a user is actually
	// working on the booking. Updated every 30s; minute-level granularity, no seconds.
	let archiveDeadline = $derived(data.existing?.archive_deadline ?? null);
	let nowTick = $state(Date.now());
	$effect(() => {
		const timer = setInterval(() => nowTick = Date.now(), 30_000);
		return () => clearInterval(timer);
	});
	function formatCountdown(msLeft: number): string {
		const totalMinutes = Math.max(0, Math.floor(msLeft / 60_000));
		const days = Math.floor(totalMinutes / (24 * 60));
		const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
		const minutes = totalMinutes % 60;
		if (days > 0) {
			return m.page_booking_archive_countdown_days({ days: String(days), hours: String(hours) });
		}
		return m.page_booking_archive_countdown_hours({ hours: String(hours), minutes: String(minutes) });
	}

	$effect(() => {
		if (data.existing) {
			startDate = data.existing.booking.start_date;
			endDate = data.existing.booking.end_date;
			title = data.existing.booking.title;
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

	function showMessage(text: string) {
		message = text;
		setTimeout(() => message = '', 4000);
	}

	// A date-change update can be hard-rejected (409) with the article_ids of
	// every unavailable item - highlight those rows using the same per-row
	// indicator as the informational post-save conflict check, so the user
	// knows which items to remove or swap.
	function flagConflictFromError(e: unknown) {
		const ids = e instanceof ApiError ? e.body?.params?.article_ids : undefined;
		if (ids) {
			conflictingIds = new Set(ids.split(','));
		}
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
				title,
				used_by_team_id: selectedUnit || ''
			});
			// Check for conflicts after date change. exclude_own_items must be
			// false here: we're revalidating the booking's own current items
			// against the (possibly unchanged) date range, not offering new
			// items to add - the default (true) would flag every held item
			// as unavailable since it excludes them from the result set.
			const available = await api.listAvailableArticles(startDate, endDate, { exclude_booking_id: bookingId, exclude_own_items: false });
			const availableIds = new Set(available.map(a => a.id));
			const conflicts = new Set(
				cartItems
					.filter(item => !availableIds.has(item.article_id))
					.map(item => item.article_id)
			);
			conflictingIds = conflicts;
			if (conflicts.size === 0) {
				showMessage(m.page_book_changes_saved());
			}
		} catch (e) {
			error = translateError(e);
			flagConflictFromError(e);
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
		} catch (e) {
			error = translateError(e);
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
		} catch (e) {
			error = translateError(e);
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
		if (!startDate) {
			error = m.error_invalid_start_date();
			return;
		}
		if (!endDate) {
			error = m.error_invalid_end_date();
			return;
		}
		if (!title.trim()) {
			error = m.error_title_is_required();
			return;
		}
		submitting = true;
		error = '';
		try {
			await api.updateBooking(bookingId, {
				start_date: startDate,
				end_date: endDate,
				title,
				used_by_team_id: selectedUnit || ''
			});
			const booking = await api.submitBooking(bookingId);
			cart.clear();
			submitted = true;
			message = booking.status === 'confirmed' ? m.page_book_booking_confirmed() : m.page_book_booking_submitted();
		} catch (e) {
			error = translateError(e);
			flagConflictFromError(e);
		}
		submitting = false;
	}

	async function cancelBooking() {
		if (!bookingId) return;
		if (!confirm(m.common_confirm())) return;
		try {
			await api.cancelBooking(bookingId);
			cart.clear();
			window.location.href = '/';
		} catch (e) {
			error = translateError(e);
		}
	}
</script>

<div class="max-w-4xl mx-auto p-4">
	{#if !data.existing}
		<!-- Create mode -->
		<h1 class="text-heading-sm font-bold mb-4">{m.page_book_heading()}</h1>

		{#if createError}
			<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{createError}</div>
		{/if}

		<div class="grid grid-cols-2 sm:flex sm:flex-wrap gap-3 mb-4">
			<label class="flex flex-col gap-1">
				<span class="text-sm">{m.page_book_start_date()}</span>
				<input type="date" bind:value={newStartDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">{m.page_book_end_date()}</span>
				<input type="date" bind:value={newEndDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">{m.page_book_title()}</span>
				<input type="text" bind:value={newTitle} placeholder={m.page_book_title_placeholder()} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">{m.page_book_booked_for()}</span>
				<select bind:value={newUnit} class="border rounded px-3 py-2">
					{#each userTeams.filter(u => myTeamSet.has(u.name)) as unit}
						<option value={unit.id}>{unit.name} ({msg(`team_access_${unit.access_level}`) ?? unit.access_level})</option>
					{/each}
					{#if canPersonal}
						<option value="">{m.page_book_personal()}</option>
					{/if}
					{#if isManager}
						{@const otherTeams = userTeams.filter(u => !myTeamSet.has(u.name))}
						{#if otherTeams.length > 0}
							<option disabled>───</option>
							{#each otherTeams as unit}
								<option value={unit.id}>{unit.name} ({msg(`team_access_${unit.access_level}`) ?? unit.access_level})</option>
							{/each}
						{/if}
					{/if}
				</select>
			</label>
		</div>

		<!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
		<scout-button
			type="button"
			variant="primary"
			onclick={createBooking}
			disabled={!newStartDate || !newEndDate || !newTitle.trim() || creating ? true : undefined}
		>
			{creating ? '...' : m.page_book_btn_create()}
		</scout-button>

	{:else if submitted}
		<div class="bg-green-50 border border-green-200 rounded p-4">
			<p class="font-medium text-green-800">{message}</p>
			<a href="/bookings/{bookingId}" class="underline text-green-700">{m.page_book_view_booking()}</a>
		</div>

	{:else}
		<!-- Cart management mode -->
		<div class="flex items-center justify-between gap-3 mb-4">
			<h1 class="text-heading-sm font-bold">{m.page_book_your_booking()}</h1>
			<scout-button type="link" href="/bookings/{bookingId}" variant="outlined">{m.page_book_view_booking()}</scout-button>
		</div>

		{#if message}
			<div class="bg-green-50 border border-green-200 rounded p-3 mb-4 text-green-800 text-sm">{message}</div>
		{/if}

		{#if error}
			<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{error}</div>
		{/if}

		{#if archiveDeadline && (data.existing?.booking.status === 'draft' || data.existing?.booking.status === 'rejected')}
			{@const msLeft = new Date(archiveDeadline).getTime() - nowTick}
			{#if msLeft > 0}
				<div class="border rounded p-3 mb-4 text-sm {msLeft < 24 * 3600_000 ? 'bg-red-50 border-red-300 text-red-900' : 'bg-amber-50 border-amber-300 text-amber-900'}">
					<p class="font-medium mb-1">{formatCountdown(msLeft)}</p>
					<p>{data.existing?.booking.status === 'draft' ? m.page_booking_archive_hint_draft() : m.page_booking_archive_hint_rejected()}</p>
				</div>
			{/if}
		{/if}

		{#if hasConflicts}
			<div class="bg-orange-50 border border-orange-200 rounded p-3 mb-4 text-orange-800 text-sm">
				<p class="font-medium mb-1">{m.page_book_date_conflict({ count: String(conflictingIds.size) })}</p>
				<p>{m.page_book_date_conflict_hint()}</p>
			</div>
		{/if}

		<!-- Dates, team, title -->
		<div class="grid grid-cols-2 sm:flex sm:flex-wrap gap-3 mb-3">
			<label class="flex flex-col gap-1">
				<span class="text-sm">{m.page_book_start_date()}</span>
				<input type="date" bind:value={startDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">{m.page_book_end_date()}</span>
				<input type="date" bind:value={endDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">{m.page_book_title()}</span>
				<input type="text" bind:value={title} placeholder={m.page_book_title_placeholder()} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1 col-span-2">
				<span class="text-sm">{m.page_book_booked_for()}</span>
				<select bind:value={selectedUnit} class="border rounded px-3 py-2">
					{#each userTeams.filter(u => myTeamSet.has(u.name)) as unit}
						<option value={unit.id}>{unit.name} ({msg(`team_access_${unit.access_level}`) ?? unit.access_level})</option>
					{/each}
					{#if canPersonal}
						<option value="">{m.page_book_personal()}</option>
					{/if}
					{#if isManager}
						{@const otherTeams = userTeams.filter(u => !myTeamSet.has(u.name))}
						{#if otherTeams.length > 0}
							<option disabled>───</option>
							{#each otherTeams as unit}
								<option value={unit.id}>{unit.name} ({msg(`team_access_${unit.access_level}`) ?? unit.access_level})</option>
							{/each}
						{/if}
					{/if}
				</select>
			</label>
		</div>

		<div class="flex gap-2 mb-6">
			<!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
			<scout-button
				type="button"
				variant="outlined"
				onclick={saveDetails}
				disabled={saving ? true : undefined}
			>
				{saving ? '...' : m.page_book_btn_save_changes()}
			</scout-button>
		</div>

		<!-- Item list -->
		{#if cartItems.length > 0}
			<h2 class="font-medium mb-2">{m.page_booking_items_heading({ count: String(cartItems.length) })}</h2>
			<BookingItemsList items={cartItems} editable conflictingIds={conflictingIds} onRemove={removeFromCart} onAddOne={addOneToCart} onRemoveOne={removeOneFromCart} />
		{:else}
			<p class="text-sm text-neutral-500 mb-4">{m.page_book_items_empty()}</p>
		{/if}

		{#if bookingId}
			<BookingCommentThread {bookingId} />
		{/if}

		<!-- Primary actions -->
		<div class="flex flex-wrap gap-3 mt-6">
			<scout-button type="link" href="/browse" variant="outlined">{m.page_book_btn_add_items()}</scout-button>
			<!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
			<scout-button
				type="button"
				variant="primary"
				onclick={submitBooking}
				disabled={cartItems.length === 0 || !startDate || !endDate || hasConflicts || submitting ? true : undefined}
			>
				{submitting ? '...' : m.page_booking_btn_submit()}
			</scout-button>
		</div>

		<!-- Cancel - separated so it's not accidentally tapped -->
		{#if cancellable}
			<div class="mt-6 pt-4 border-t">
				<!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
				<scout-button type="button" variant="danger" size="large" onclick={cancelBooking}>
					{m.page_book_btn_cancel()}
				</scout-button>
			</div>
		{/if}
	{/if}
</div>
