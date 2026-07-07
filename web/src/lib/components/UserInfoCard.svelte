<script lang="ts">
	import { createApiClient, type UserInfo } from '$lib/api/client';
	import UserAvatar from '$lib/components/UserAvatar.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import { msg } from '$lib/msg';

	interface Props {
		userId: string;
		open: boolean;
		contextBookingId?: string;
	}

	let { userId, open = $bindable(), contextBookingId }: Props = $props();

	const api = createApiClient();

	let info = $state<UserInfo | null>(null);
	let loading = $state(false);
	let error = $state('');

	async function load() {
		loading = true;
		error = '';
		info = null;
		try {
			info = await api.getUserInfo(userId);
		} catch {
			error = m.user_info_card_error();
		}
		loading = false;
	}

	$effect(() => {
		if (open) load();
	});

	function close() {
		open = false;
	}

	const highlightedTeamId = $derived(
		contextBookingId
			? info?.open_bookings.find((b) => b.id === contextBookingId)?.used_by_team_id
			: undefined
	);

	// draft/rejected bookings are only ever fetched by the API when the viewer is a manager.
	const managerOnlyStatuses = new Set(['draft', 'rejected']);
	const visibleBookings = $derived(info?.open_bookings.filter((b) => !managerOnlyStatuses.has(b.status)) ?? []);
	const managerOnlyBookings = $derived(info?.open_bookings.filter((b) => managerOnlyStatuses.has(b.status)) ?? []);
</script>

{#snippet bookingRow(booking: NonNullable<typeof info>['open_bookings'][number])}
	<li class="text-sm">
		<a href={`/bookings/${booking.id}`} class="block hover:underline">
			<div class="flex items-center gap-2">
				{#if booking.team_name}
					<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">{booking.team_name}</span>
				{:else if booking.used_by_external}
					<span class="text-xs bg-neutral-50 text-neutral-600 px-1.5 py-0.5 rounded">{booking.used_by_external}</span>
				{/if}
				<span class="text-blue-700">{booking.start_date} - {booking.end_date}</span>
				<span class="text-neutral-500 text-xs">({msg('booking_status_' + booking.status)})</span>
			</div>
			{#if booking.title}
				<p class="text-neutral-500 text-xs truncate">{booking.title}</p>
			{/if}
		</a>
	</li>
{/snippet}

{#if open}
	<button
		type="button"
		class="fixed inset-0 z-40 bg-black/60"
		aria-label={m.btn_close()}
		onclick={close}
	></button>

	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
		<div
			class="bg-white rounded-2xl shadow-xl w-full max-w-lg max-h-[calc(100dvh-2rem)] overflow-y-auto p-6 space-y-5 pointer-events-auto"
			role="dialog"
			aria-modal="true"
		>
			{#if loading}
				<p class="text-sm text-neutral-500">{m.user_info_card_loading()}</p>
			{:else if error}
				<p class="text-sm text-red-600">{error}</p>
			{:else if info}
				<div class="flex items-start justify-between gap-2">
					<div class="flex items-center gap-3">
						<UserAvatar name={info.name} picture={info.picture} size={64} />
						<p class="font-semibold text-sm">{info.name}</p>
					</div>
					<button
						type="button"
						onclick={close}
						aria-label={m.btn_close()}
						class="text-neutral-400 hover:text-neutral-700 shrink-0 text-lg leading-none"
					>×</button>
				</div>

				{#if info.notification_email}
					<div>
						<p class="text-xs font-medium text-neutral-500">{m.user_info_card_notification_email()}</p>
						<p class="text-sm">{info.notification_email}</p>
					</div>
				{/if}

				<div>
					<p class="text-xs font-medium text-neutral-500 mb-1">{m.user_info_card_teams_heading()}</p>
					{#if info.teams.length === 0}
						<p class="text-sm text-neutral-500">{m.user_info_card_no_teams()}</p>
					{:else}
						<ul class="space-y-1">
							{#each info.teams as team}
								<li
									class="text-sm flex items-center justify-between gap-2 rounded px-2 py-1 {team.id === highlightedTeamId ? 'bg-blue-50 font-medium' : ''}"
								>
									<span>{team.name} <span class="text-neutral-400">({msg('team_type_' + team.type)})</span></span>
									<span class="text-neutral-500 text-xs">{msg('team_access_' + team.access_level)}</span>
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<div>
					<p class="text-xs font-medium text-neutral-500 mb-1">{m.user_info_card_open_bookings_heading()}</p>
					{#if visibleBookings.length === 0}
						<p class="text-sm text-neutral-500">{m.user_info_card_no_open_bookings()}</p>
					{:else}
						<ul class="space-y-2">
							{#each visibleBookings as booking}
								{@render bookingRow(booking)}
							{/each}
						</ul>
					{/if}
				</div>

				{#if managerOnlyBookings.length > 0}
					<div>
						<p class="text-xs font-medium text-amber-700 mb-1">{m.user_info_card_manager_only_heading()}</p>
						<ul class="space-y-2">
							{#each managerOnlyBookings as booking}
								{@render bookingRow(booking)}
							{/each}
						</ul>
					</div>
				{/if}

				<div>
					<p class="text-xs font-medium text-neutral-500 mb-1">{m.user_info_card_issues_heading()}</p>
					{#if info.issues.length === 0}
						<p class="text-sm text-neutral-500">{m.user_info_card_no_issues()}</p>
					{:else}
						<ul class="space-y-1">
							{#each info.issues as issue}
								<li class="text-sm">
									<a href={`/issues/${issue.id}`} class="text-blue-700 hover:underline">{issue.title}</a>
									<span class="text-neutral-500 text-xs">({msg('issue_status_' + issue.status)})</span>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}
