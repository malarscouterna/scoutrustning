<script lang="ts">
	import { canBook, isManager as checkManager } from '$lib/user';
	import { cart } from '$lib/stores/cart.svelte';
	import BookingCard from '$lib/components/BookingCard.svelte';
	import IssueCard from '$lib/components/IssueCard.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	let showBook = $derived(canBook(data.user));
	let mgr = $derived(checkManager(data.user));

	let drafts = $derived(data.bookings.filter(b => b.status === 'draft'));
	let pendingIds = $derived(new Set(data.pendingApprovals.map(b => b.id)));
	let active = $derived(data.bookings.filter(b => ['submitted', 'approved', 'confirmed', 'picked_up'].includes(b.status) && !pendingIds.has(b.id)));

	function activateDraft(bookingId: string) {
		cart.activate(bookingId);
	}

	let managerIssuesLimited = $derived((data.managerIssues ?? []).slice(0, 5));
</script>

<div class="max-w-4xl mx-auto p-4">
	<div class="flex flex-wrap gap-2 mb-6">
		{#if showBook}
			<scout-button type="link" href="/book" variant="primary">Boka utrustning</scout-button>
		{/if}
		<scout-button type="link" href="/browse" variant="outlined">Visa utrustning</scout-button>
		<scout-button type="link" href="/profile" variant="outlined">Inställningar</scout-button>
		<scout-button type="link" href="/guide" variant="outlined">Användarguide</scout-button>
		<scout-button type="link" href="/issues/new" variant="primary">Felanmälan</scout-button>
	</div>

	<div class="grid md:grid-cols-2 gap-6">
		<section class="min-w-0">
			<div class="flex items-center justify-between mb-3">
				<h2 class="font-bold text-lg">Bokningar</h2>
				<a href="/bookings" class="text-sm text-blue-700 hover:underline">Visa alla →</a>
			</div>

			{#if drafts.length > 0}
				<h3 class="text-sm font-medium text-neutral-500 mb-1">Utkast</h3>
				<div class="space-y-1 mb-3">
					{#each drafts as booking}
						<a href="/book?id={booking.id}" onclick={() => activateDraft(booking.id)} class="block border border-l-4 border-l-neutral-300 rounded px-4 py-3 hover:bg-neutral-50 border-dashed">
							<div class="flex flex-wrap items-center justify-between gap-1">
								<span class="font-medium">{booking.start_date} — {booking.end_date}</span>
								{#if booking.team_name}
									<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">{booking.team_name}</span>
								{/if}
							</div>
							{#if booking.notes}
								<p class="text-sm text-neutral-500 mt-1 truncate">{booking.notes}</p>
							{/if}
						</a>
					{/each}
				</div>
			{/if}

			{#if mgr && data.pendingApprovals.length > 0}
				<h3 class="text-sm font-medium text-orange-700 mb-1">
					Väntar på godkännande ({data.pendingApprovals.length})
				</h3>
				<div class="space-y-1 mb-3">
					{#each data.pendingApprovals as booking}
						<BookingCard {booking} href="/bookings/{booking.id}" />
					{/each}
				</div>
			{/if}

			{#if active.length > 0}
				<h3 class="text-sm font-medium text-neutral-500 mb-1">Aktiva</h3>
				<div class="space-y-1 mb-3">
					{#each active as booking}
						<BookingCard {booking} href="/bookings/{booking.id}" />
					{/each}
				</div>
			{/if}

			{#if data.bookings.length === 0}
				<p class="text-sm text-neutral-500">Inga bokningar ännu.</p>
			{/if}
		</section>

		<section class="min-w-0">
			<div class="flex items-center justify-between mb-3">
				<h2 class="font-bold text-lg">Ärenden</h2>
				<a href="/issues" class="text-sm text-blue-700 hover:underline">Visa alla →</a>
			</div>

			{#if data.myIssues.length > 0}
				<h3 class="text-sm font-medium text-neutral-500 mb-1">Mina ärenden</h3>
				<div class="space-y-1 mb-3">
					{#each data.myIssues as issue}
						<IssueCard {issue} />
					{/each}
				</div>
			{/if}

			{#if mgr && managerIssuesLimited.length > 0}
				<h3 class="text-sm font-medium text-neutral-500 mb-1">Aktiva ärenden</h3>
				<div class="space-y-1 mb-3">
					{#each managerIssuesLimited as issue}
						<IssueCard {issue} />
					{/each}
				</div>
			{/if}

			{#if data.myIssues.length === 0 && (!mgr || managerIssuesLimited.length === 0)}
				<p class="text-sm text-neutral-500">Inga ärenden att visa.</p>
			{/if}
		</section>
	</div>

</div>
