<script lang="ts">
	import { createApiClient, type Booking, type Team } from '$lib/api/client';
	import { page } from '$app/stores';
	import { hasRole, canBookPersonal } from '$lib/user';
	import { msg } from '$lib/msg';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';

	interface Props {
		source: Booking;
		onClose: () => void;
		onCopied: (newBookingId: string) => void;
	}

	let { source, onClose, onCopied }: Props = $props();

	const api = createApiClient();

	// svelte-ignore state_referenced_locally
	let title = $state(source.title);
	// svelte-ignore state_referenced_locally
	let selectedUnit = $state(source.used_by_team_id ?? '');
	let startDate = $state('');
	let endDate = $state('');
	let teams = $state<Team[]>([]);
	let saving = $state(false);
	let error = $state('');
	let copiedBookingId = $state<string | null>(null);

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let canPersonal = $derived(canBookPersonal($page.data.user));
	let myTeamSet = $derived(new Set(($page.data.user?.teams ?? []).map((t: { team_name: string }) => t.team_name)));
	let myTeams = $derived(teams.filter((u) => myTeamSet.has(u.name)));
	let otherTeams = $derived(teams.filter((u) => !myTeamSet.has(u.name)));

	$effect(() => {
		api.listTeams().then((t) => (teams = t));
	});

	async function submit() {
		if (!title.trim()) {
			error = m.error_title_is_required();
			return;
		}
		if (!startDate) {
			error = m.error_invalid_start_date();
			return;
		}
		if (!endDate) {
			error = m.error_invalid_end_date();
			return;
		}
		saving = true;
		error = '';
		try {
			// The copy endpoint itself is created with placeholder dates - we
			// immediately overwrite them via updateBooking below with the dates
			// chosen here, before the user ever sees the copy in the cart
			// builder. Update runs the same conflict-checking as any other
			// date change, so an unavailable item surfaces as a 409 here
			// rather than being silently carried over.
			if (!copiedBookingId) {
				const result = await api.copyBooking(source.id);
				copiedBookingId = result.booking.id;
			}
			await api.updateBooking(copiedBookingId, {
				title,
				used_by_team_id: selectedUnit || '',
				start_date: startDate,
				end_date: endDate
			});
			onCopied(copiedBookingId);
		} catch (e) {
			error = translateError(e);
		}
		saving = false;
	}
</script>

<div class="fixed inset-0 z-50 flex items-start justify-center pt-12 px-4">
	<button type="button" class="absolute inset-0 bg-black/40" onclick={onClose} aria-label={m.btn_close()}></button>
	<div class="relative bg-white rounded-2xl w-full max-w-md shadow-xl p-6 space-y-4">
		<h2 class="font-medium text-lg">{m.copy_booking_modal_heading()}</h2>

		{#if error}
			<div class="bg-red-50 border border-red-200 rounded p-3 text-red-800 text-sm">{error}</div>
		{/if}

		<label class="flex flex-col gap-1">
			<span class="text-sm">{m.page_book_title()}</span>
			<input type="text" bind:value={title} class="border rounded px-3 py-2" />
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-sm">{m.page_book_booked_for()}</span>
			<select bind:value={selectedUnit} class="border rounded px-3 py-2 w-full">
				{#each myTeams as unit}
					<option value={unit.id}>{unit.name} ({msg(`team_access_${unit.access_level}`) ?? unit.access_level})</option>
				{/each}
				{#if canPersonal}
					<option value="">{m.page_book_personal()}</option>
				{/if}
				{#if isManager && otherTeams.length > 0}
					<option disabled>───</option>
					{#each otherTeams as unit}
						<option value={unit.id}>{unit.name} ({msg(`team_access_${unit.access_level}`) ?? unit.access_level})</option>
					{/each}
				{/if}
			</select>
		</label>

		<div class="grid grid-cols-2 gap-3">
			<label class="flex flex-col gap-1">
				<span class="text-sm">{m.page_book_start_date()}</span>
				<input type="date" bind:value={startDate} class="border rounded px-3 py-2" />
			</label>
			<label class="flex flex-col gap-1">
				<span class="text-sm">{m.page_book_end_date()}</span>
				<input type="date" bind:value={endDate} class="border rounded px-3 py-2" />
			</label>
		</div>

		<div class="flex justify-end gap-2 pt-2">
			<button type="button" onclick={onClose} class="border rounded px-4 py-2 text-sm">{m.btn_cancel()}</button>
			<button
				type="button"
				onclick={submit}
				disabled={saving}
				class="bg-green-700 text-white rounded px-4 py-2 text-sm disabled:opacity-50"
			>
				{m.copy_booking_modal_btn_confirm()}
			</button>
		</div>
	</div>
</div>
