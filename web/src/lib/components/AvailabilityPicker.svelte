<script lang="ts">
	import { createApiClient, type AvailabilityGroup, type BookingItem, type Category, type Location } from '$lib/api/client';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';
	import { msg } from '$lib/msg';

	interface Props {
		bookingId: string;
		startDate: string;
		endDate: string;
		categories: Category[];
		locations: Location[];
		cartItems: BookingItem[];
		teamAccessLevel: string;
		userIsManager: boolean;
		onItemsChanged: () => void;
	}

	let { bookingId, startDate, endDate, categories, locations, cartItems, teamAccessLevel, userIsManager, onItemsChanged }: Props = $props();

	const api = createApiClient();

	function formatDate(iso: string): string {
		return new Date(iso).toLocaleDateString('sv', { day: 'numeric', month: 'short' });
	}

	let bookedCounts = $derived.by(() => {
		const map = new Map<string, number>();
		for (const item of cartItems) {
			const key = item.commercial_name + '||' + item.location_name;
			map.set(key, (map.get(key) ?? 0) + 1);
		}
		return map;
	});

	let stableAvailability = $state<AvailabilityGroup[]>([]);
	let selectedCategory = $state('');
	let selectedLocation = $state('');
	let searchQuery = $state('');
	let showAll = $state(false);

	// Managers default to showing all
	$effect(() => {
		if (userIsManager) showAll = true;
	});
	let quantities = $state<Record<string, number>>({});
	let error = $state('');
	let expandedDetails = $state<string | null>(null);
	let expandedInfo = $state<Set<string>>(new Set());
	let detailArticles = $state<Map<string, { articles: any[]; comments: Map<string, string> }>>(new Map());

	// Filter availability based on team access level:
	// Default: show items bookable without approval + with availability > 0
	// Show all: show everything including approval-required and fully booked
	let filteredAvailability = $derived.by(() => {
		let items = stableAvailability;
		if (searchQuery) {
			items = items.filter(g => g.commercial_name.toLowerCase().includes(searchQuery.toLowerCase()));
		}
		if (!showAll) {
			items = items.filter(g => {
				// Hide fully booked
				const booked = bookedCounts.get(g.commercial_name + '||' + g.location_name) ?? 0;
				if (g.available_count <= 0 && booked <= 0) return false;
				// Hide items that need approval for this team
				if (g.approval_level === 'high') return false;
				if (g.approval_level === 'low' && (teamAccessLevel === 'view' || teamAccessLevel === 'book')) return false;
				return true;
			});
		}
		return items;
	});

	// Load on mount
	loadAvailability();

	async function toggleDetails(group: AvailabilityGroup) {
		const key = group.commercial_name + '||' + group.location_name;
		if (expandedDetails === key) {
			expandedDetails = null;
			return;
		}
		expandedDetails = key;
		if (detailArticles.has(key)) return;

		try {
			const articles = await api.listAvailableArticles(startDate, endDate, {
				exclude_booking_id: bookingId,
				commercial_name: group.commercial_name
			});
			const flagged = articles.filter(a => a.status !== 'ok');
			const comments = new Map<string, string>();

			await Promise.all(flagged.map(async (a) => {
				try {
					const { events } = await api.listArticleEvents(a.id, 1);
					if (events[0]?.description) comments.set(a.id, events[0].description);
				} catch { /* ignore */ }
			}));

			const next = new Map(detailArticles);
			next.set(key, { articles: flagged, comments });
			detailArticles = next;
		} catch { /* ignore */ }
	}

	export async function loadAvailability() {
		if (!startDate || !endDate) return;
		error = '';
		try {
			const fresh = await api.checkAvailability(startDate, endDate, {
				category_id: selectedCategory || undefined,
				location_id: selectedLocation || undefined,
			});
			if (stableAvailability.length === 0) {
				stableAvailability = fresh;
			} else {
				const freshMap = new Map<string, AvailabilityGroup>();
				for (const g of fresh) {
					freshMap.set(g.commercial_name + '||' + g.location_name, g);
				}
				stableAvailability = stableAvailability.map(g => {
					const key = g.commercial_name + '||' + g.location_name;
					return freshMap.get(key) ?? { ...g, available_count: 0 };
				});
			}
		} catch (e) {
			error = translateError(e);
		}
	}

	function onFilterChange() {
		stableAvailability = [];
		loadAvailability();
	}

	function hasExpandableContent(g: AvailabilityGroup): boolean {
		return (g.image_ids?.length ?? 0) > 0 || !!g.description || !!g.instructions;
	}

	function toggleInfo(key: string) {
		const next = new Set(expandedInfo);
		if (next.has(key)) next.delete(key); else next.add(key);
		expandedInfo = next;
	}

	async function addToCart(commercialName: string, locationName: string) {
		const key = commercialName + '||' + locationName;
		const qty = quantities[key] || 1;
		error = '';
		try {
			await api.addBookingItems(bookingId, commercialName, qty, locationName);
			quantities[key] = 1;
			await loadAvailability();
			onItemsChanged();
		} catch (e) {
			error = translateError(e);
		}
	}
</script>

{#if error}
	<div class="bg-red-50 border border-red-200 rounded p-3 mb-4 text-red-800 text-sm">{error}</div>
{/if}

<div class="flex flex-wrap gap-2 mb-3">
	<input
		type="search"
		placeholder={m.availability_search_placeholder()}
		bind:value={searchQuery}
		class="border rounded px-3 py-2 text-sm flex-1 min-w-48"
	/>
	<select bind:value={selectedCategory} onchange={onFilterChange} class="border rounded px-3 py-2 text-sm">
		<option value="">{m.availability_all_categories()}</option>
		{#each categories as cat}
			<option value={cat.id}>{cat.name}</option>
		{/each}
	</select>
	<select bind:value={selectedLocation} onchange={onFilterChange} class="border rounded px-3 py-2 text-sm">
		<option value="">{m.availability_all_locations()}</option>
		{#each locations as loc}
			<option value={loc.id}>{loc.name}</option>
		{/each}
	</select>
	<label class="flex items-center gap-1.5 text-sm">
		<input type="checkbox" bind:checked={showAll} />
		{m.availability_show_all()}
	</label>
</div>

<div class="space-y-1 mb-4">
	{#each filteredAvailability as group}
		{@const key = group.commercial_name + '||' + group.location_name}
		{@const hasFlags = group.reported_usable_count > 0 || group.incoming_count > 0 || group.under_repair_count > 0}
		{@const booked = bookedCounts.get(key) ?? 0}
		{@const expandable = hasExpandableContent(group)}
		{@const infoExpanded = expandedInfo.has(key)}
		<div class="border rounded">
			<div class="flex flex-wrap items-center justify-between gap-2 px-4 py-2">
				<button type="button" onclick={() => expandable && toggleInfo(key)} class="min-w-0 text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
					<span class="font-medium">{group.commercial_name}</span>
					{#if expandable}<span class="text-xs text-neutral-400 ml-1">{infoExpanded ? '▲' : '▼'}</span>{/if}
					<span class="text-xs text-neutral-500 ml-2">{group.category_name} · {group.location_name}</span>
					{#if group.approval_level === 'high'}
						<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">{m.report_issue_requires_approval()}</span>
					{:else if group.approval_level === 'low'}
						{#if teamAccessLevel === 'view' || teamAccessLevel === 'book'}
							<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">{m.report_issue_requires_approval()}</span>
						{:else}
							<span class="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded ml-1">{m.availability_pre_approved()}</span>
						{/if}
					{/if}
				</button>
				<div class="flex items-center gap-2">
					<div class="text-sm text-neutral-600 text-right">
						{#if booked > 0}
							<span class="text-green-700 font-medium">{m.availability_booked_count({ booked: String(booked), total: String(booked + group.available_count) })}</span>
						{:else}
							<span>{m.availability_remaining_count({ count: String(group.available_count) })}</span>
						{/if}
						{#if hasFlags}
							<button onclick={() => toggleDetails(group)} class="text-xs text-neutral-400 ml-1 underline cursor-pointer">
								({#if group.reported_usable_count > 0}<span class="text-orange-600">{m.availability_reported_count({ count: String(group.reported_usable_count) })}</span>{/if}{#if group.incoming_count > 0}{#if group.reported_usable_count > 0}, {/if}<span class="text-blue-600">{m.availability_incoming_count({ count: String(group.incoming_count) })}</span>{/if}{#if group.under_repair_count > 0}{#if group.reported_usable_count > 0 || group.incoming_count > 0}, {/if}<span class="text-neutral-600">{m.availability_under_repair_count({ count: String(group.under_repair_count) })}</span>{/if})
							</button>
						{/if}
					</div>
					<input
						type="number"
						min="1"
						max={group.available_count}
						bind:value={quantities[key]}
						class="border rounded w-14 px-2 py-1 text-sm text-center"
						placeholder="1"
					/>
					<button
						onclick={() => addToCart(group.commercial_name, group.location_name)}
						disabled={group.available_count === 0}
						class="bg-blue-700 text-white text-sm px-3 py-1 rounded disabled:opacity-50"
					>
						{m.btn_add()}
					</button>
				</div>
			</div>
			{#if infoExpanded}
				<div class="px-4 py-2 border-t space-y-2 text-xs text-neutral-600">
					{#if (group.image_ids?.length ?? 0) > 0}
						<ImageViewer imageIds={group.image_ids} alt={group.commercial_name} commercialName={group.commercial_name} locationId={group.location_id} />
					{/if}
					{#if group.description}
						<div>
							<span class="font-medium text-neutral-500">{m.lbl_description_colon()}</span>
							<p class="mt-0.5">{group.description}</p>
						</div>
					{/if}
					{#if group.instructions}
						<div>
							<span class="font-medium text-neutral-500">{m.lbl_instructions_colon()}</span>
							<p class="mt-0.5">{group.instructions}</p>
						</div>
					{/if}
				</div>
			{/if}
			{#if expandedDetails === key}
				{@const detail = detailArticles.get(key)}
				<div class="border-t px-4 py-2 bg-neutral-50 text-sm space-y-1">
					{#if !detail}
						<p class="text-xs text-neutral-400">{m.btn_loading()}</p>
					{:else if detail.articles.length === 0}
						<p class="text-xs text-neutral-400">{m.availability_no_details()}</p>
					{:else}
						{#each detail.articles as article}
							<div class="flex flex-wrap items-start gap-2 text-xs">
								<span class="px-1.5 py-0.5 rounded
									{article.status === 'reported_usable' ? 'bg-orange-100 text-orange-700' : article.status === 'incoming' ? 'bg-blue-50 text-blue-700' : article.status === 'under_repair' ? 'bg-neutral-100 text-neutral-700' : 'bg-neutral-100'}">
									{article.status === 'reported_usable' ? m.availability_reported() : article.status === 'incoming' ? m.article_status_incoming() : article.status === 'under_repair' ? m.article_status_under_repair() : article.status}
								</span>
								{#if article.common_name !== group.commercial_name}
									<span class="text-neutral-600">{article.common_name}</span>
								{/if}
								{#if article.expected_available_date}
									<span class="text-blue-600">{article.status === 'incoming' ? m.availability_expected_delivery() : m.availability_expected_ready()} {formatDate(article.expected_available_date)}</span>
								{/if}
								{#if detail.comments.get(article.id)}
									<span class="text-neutral-500 italic">“{detail.comments.get(article.id)}”</span>
								{/if}
							</div>
						{/each}
					{/if}
				</div>
			{/if}
		</div>
	{/each}
</div>
