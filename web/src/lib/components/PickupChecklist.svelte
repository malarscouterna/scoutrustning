<script lang="ts">
	import { createApiClient, type BookingItem, type Issue } from '$lib/api/client';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import ReportIssueSheet from '$lib/components/ReportIssueSheet.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';

	interface Props {
		bookingId: string;
		items: BookingItem[];
		startDate: string;
		endDate: string;
		onUpdate: () => Promise<BookingItem[]>;
	}

	let { bookingId, items, startDate, endDate, onUpdate }: Props = $props();

	const api = createApiClient();

	let error = $state('');
	let loading = $state(false);
	let expandedGroups = $state<Set<string>>(new Set());
	let issuesByArticle = $state<Record<string, Issue[]>>({});

	// Issue sheet
	let issueSheetArticle = $state<{ id: string; name: string; isQuantityTracked?: boolean; groupTotal?: number } | null>(null);
	// Set to an article_id when felanmäl is opened for an individual item; cleared after auto-swap triggered
	let pendingSwapArticleId = $state<string | null>(null);

	// Swap
	interface SwapCandidate {
		id: string;
		common_name: string;
		location_name: string;
		place: string;
		status: string;
		expected_available_date: string | null;
		is_current?: boolean;
	}
	let swappingItemId = $state<string | null>(null);
	let swapCandidates = $state<SwapCandidate[]>([]);
	let selectedSwapArticle = $state('');

	const pickupLabels = $derived<Record<string, string>>({
		picked_up: m.pickup_status_picked_up(),
		swapped: m.pickup_status_swapped()
	});

	function isReported(status: string | null | undefined) {
		return status === 'reported_usable' || status === 'reported_unusable' || status === 'reported_missing';
	}
	function isUnusable(status: string | null | undefined) {
		return status === 'reported_unusable' || status === 'reported_missing';
	}

	type StatusCategory = 'ok' | 'reported_usable' | 'reported_unusable';

	function statusCategory(articleStatus: string | null | undefined): StatusCategory {
		if (isUnusable(articleStatus)) return 'reported_unusable';
		if (articleStatus === 'reported_usable') return 'reported_usable';
		return 'ok';
	}

	interface QuantitySubGroup {
		commercialName: string;
		locationName: string;
		place: string;
		category: StatusCategory;
		key: string;
		items: BookingItem[];
	}

	interface TrackedGroup {
		commercialName: string;
		imageIds: string[];
		locationId: string;
		description: string;
		instructions: string;
		items: BookingItem[];
	}

	let trackedItems = $derived(items.filter((i) => i.individually_tracked));
	let quantitySubGroups = $derived.by(() => {
		const map = new Map<string, QuantitySubGroup>();
		for (const item of items) {
			if (item.individually_tracked) continue;
			const cat = statusCategory(item.article_status);
			const key = `${item.commercial_name}|${item.location_name}|${cat}`;
			const existing = map.get(key);
			if (existing) existing.items.push(item);
			else map.set(key, {
				commercialName: item.commercial_name,
				locationName: item.location_name,
				place: item.place,
				category: cat,
				key,
				items: [item]
			});
		}
		// Sort: ok first, then reported_usable, then reported_unusable
		const order: StatusCategory[] = ['ok', 'reported_usable', 'reported_unusable'];
		return [...map.values()].sort((a, b) => order.indexOf(a.category) - order.indexOf(b.category));
	});

	let trackedGroups = $derived.by(() => {
		const map = new Map<string, TrackedGroup>();
		for (const item of trackedItems) {
			const existing = map.get(item.commercial_name);
			if (existing) existing.items.push(item);
			else map.set(item.commercial_name, {
				commercialName: item.commercial_name,
				imageIds: item.image_ids ?? [],
				locationId: item.location_id,
				description: item.article_description ?? '',
				instructions: item.article_instructions ?? '',
				items: [item]
			});
		}
		return [...map.values()];
	});

	function toggleExpand(key: string) {
		const next = new Set(expandedGroups);
		if (next.has(key)) next.delete(key); else next.add(key);
		expandedGroups = next;
	}

	function hasExpandable(imageIds: string[], desc: string, instr: string): boolean {
		return imageIds.length > 0 || !!desc || !!instr;
	}

	let checkedCount = $derived(items.filter((i) => i.pickup_status !== null).length);

	async function markPickup(itemId: string, status: string) {
		error = '';
		try {
			await api.updateItemPickup(bookingId, itemId, status);
			await onUpdate();
		} catch (e) {
			error = translateError(e);
		}
	}

	async function markQuantityGroup(group: QuantitySubGroup, pickedCount: number) {
		error = '';
		try {
			const extraNeeded = pickedCount - group.items.length;
			if (extraNeeded > 0) {
				await api.addBookingItems(bookingId, group.commercialName, extraNeeded, group.locationName);
				const freshItems = await onUpdate();
				const updatedItems = freshItems.filter(
					(i) => !i.individually_tracked &&
						i.commercial_name === group.commercialName &&
						i.location_name === group.locationName &&
						statusCategory(i.article_status) === 'ok'
				);
				for (let i = 0; i < updatedItems.length; i++) {
					await api.updateItemPickup(bookingId, updatedItems[i].id, i < pickedCount ? 'picked_up' : '');
				}
				await onUpdate();
			} else {
				for (let i = 0; i < group.items.length; i++) {
					await api.updateItemPickup(bookingId, group.items[i].id, i < pickedCount ? 'picked_up' : '');
				}
				await onUpdate();
			}
		} catch (e) {
			error = translateError(e);
		}
	}

	async function resetQuantityGroup(group: QuantitySubGroup) {
		error = '';
		try {
			for (const item of group.items) {
				if (item.pickup_status) await api.updateItemPickup(bookingId, item.id, '');
			}
			await onUpdate();
		} catch (e) {
			error = translateError(e);
		}
	}

	async function removeFromBooking(group: QuantitySubGroup) {
		error = '';
		loading = true;
		try {
			for (const item of group.items) {
				await api.removeBookingItem(bookingId, item.id);
			}
			await onUpdate();
		} catch (e) {
			error = translateError(e);
		} finally {
			loading = false;
		}
	}

	async function loadIssuesForArticle(articleId: string) {
		if (issuesByArticle[articleId]) return;
		try {
			const issues = await api.listIssues({ article_id: articleId, status: 'open' });
			issuesByArticle = { ...issuesByArticle, [articleId]: issues };
		} catch {
			issuesByArticle = { ...issuesByArticle, [articleId]: [] };
		}
	}

	async function startSwap(item: BookingItem) {
		error = '';
		swappingItemId = item.id;
		selectedSwapArticle = '';
		try {
			const result = await api.listAvailableArticles(startDate, endDate, {
				exclude_booking_id: bookingId,
				commercial_name: item.commercial_name
			});
			const available = Array.isArray(result) ? result : [];
			const current: SwapCandidate = {
				id: item.article_id,
				common_name: item.common_name,
				location_name: item.location_name,
				place: item.place,
				status: item.article_status ?? 'ok',
				expected_available_date: item.article_expected_available_date ?? null,
				is_current: true
			};
			swapCandidates = [current, ...available];
		} catch (e) {
			error = translateError(e);
			swappingItemId = null;
		}
	}

	function cancelSwap() {
		swappingItemId = null;
		swapCandidates = [];
		selectedSwapArticle = '';
	}

	async function confirmSwap(itemId: string, currentArticleId: string) {
		if (!selectedSwapArticle) return;
		error = '';
		loading = true;
		try {
			if (selectedSwapArticle === currentArticleId) {
				await api.updateItemPickup(bookingId, itemId, 'picked_up');
			} else {
				await api.swapItem(bookingId, itemId, selectedSwapArticle);
			}
			await onUpdate();
			cancelSwap();
		} catch (e) {
			error = translateError(e);
		} finally {
			loading = false;
		}
	}

	let quantityInputs = $state<Record<string, number>>({});

	function groupPickedCount(group: QuantitySubGroup): number {
		return group.items.filter((i) => i.pickup_status === 'picked_up' || i.pickup_status === 'swapped').length;
	}

	function groupIsDone(group: QuantitySubGroup): boolean {
		return group.items.every((i) => i.pickup_status !== null);
	}

	function groupHasAnyPickedUp(group: QuantitySubGroup): boolean {
		return group.items.some((i) => i.pickup_status !== null);
	}

	async function onReported(articleId: string) {
		const freshItems = await onUpdate();
		if (pendingSwapArticleId === articleId) {
			pendingSwapArticleId = null;
			const item = freshItems.find((i) => i.article_id === articleId && !i.pickup_status);
			if (item) await startSwap(item);
		}
	}
</script>

{#snippet infoBlock(imageIds: string[], commercialName: string, locationId: string, description: string, instructions: string)}
	<div class="px-4 py-2 border-t space-y-2 text-xs text-neutral-600">
		{#if imageIds.length > 0}
			<ImageViewer {imageIds} alt={commercialName} {commercialName} {locationId} />
		{/if}
		{#if description}
			<div>
				<span class="font-medium text-neutral-500">{m.lbl_description_colon()}</span>
				<p class="mt-0.5">{description}</p>
			</div>
		{/if}
		{#if instructions}
			<div>
				<span class="font-medium text-neutral-500">{m.lbl_instructions_colon()}</span>
				<p class="mt-0.5">{instructions}</p>
			</div>
		{/if}
	</div>
{/snippet}

{#if error}
	<div class="bg-red-50 border border-red-200 rounded p-3 mb-3 text-red-800 text-sm">{error}</div>
{/if}

<p class="text-sm text-neutral-500 mb-3">{m.pickup_checked_label()} {checkedCount} / {items.length}</p>

<div class="space-y-1">
	<!-- Quantity-tracked sub-groups (split by status category) -->
	{#each quantitySubGroups as group}
		{@const picked = groupPickedCount(group)}
		{@const done = groupIsDone(group)}
		{@const anyPickedUp = groupHasAnyPickedUp(group)}
		{@const rep = group.items[0]}
		{@const qImageIds = rep?.image_ids ?? []}
		{@const expandable = group.category === 'reported_usable' || hasExpandable(qImageIds, rep?.article_description ?? '', rep?.article_instructions ?? '')}
		{@const expanded = expandedGroups.has(group.key)}
		{@const isUnusableGroup = group.category === 'reported_unusable'}
		{@const isUsableGroup = group.category === 'reported_usable'}
		<div class="border rounded"
			class:bg-green-50={done && picked > 0 && !isUnusableGroup && !isUsableGroup}
			class:bg-orange-50={(done && picked === 0 && !isUnusableGroup) || isUsableGroup}
			class:bg-red-50={isUnusableGroup}
		>
			<div class="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
				<button type="button"
					onclick={() => {
						if (isUsableGroup && rep) loadIssuesForArticle(rep.article_id);
						if (expandable) toggleExpand(group.key);
					}}
					class="flex-1 min-w-[8rem] text-left"
					class:cursor-pointer={expandable}
					class:cursor-default={!expandable}
				>
					<div class="font-medium text-sm">
						{group.commercialName}
						{#if isUnusableGroup}
							<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">{m.report_issue_unavailable()}</span>
						{:else if isUsableGroup}
							<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">{m.availability_reported()}</span>
						{/if}
						{#if expandable}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}
					</div>
					<div class="text-xs text-neutral-500">
						{group.locationName}{group.place ? ` · ${group.place}` : ''}
						· {group.items.length} st
					</div>
				</button>

				<div class="flex flex-wrap items-center gap-2">
					{#if isUnusableGroup}
						<!-- Cannot be picked up -->
					{:else if isUsableGroup}
						<!-- reported_usable: pickup or remove -->
						{#if !done}
							<button
								onclick={() => markQuantityGroup(group, group.items.length)}
								class="text-xs bg-orange-600 text-white px-3 py-1 rounded"
								disabled={loading}
							>{m.pickup_btn_pick_anyway()}</button>
							<button
								onclick={() => removeFromBooking(group)}
								class="text-xs text-red-600 underline"
								disabled={loading}
							>{m.pickup_btn_remove()}</button>
						{:else}
							<span class="text-sm font-medium text-orange-800">
								{picked} / {group.items.length} {m.pickup_count_picked()}
							</span>
							<button onclick={() => resetQuantityGroup(group)} class="text-xs text-neutral-400 hover:text-neutral-600">{m.btn_undo()}</button>
						{/if}
					{:else}
						<!-- ok group: count picker, show partial state if any picked up -->
						{#if anyPickedUp}
							<span class="text-sm font-medium" class:text-green-800={picked > 0} class:text-orange-800={picked === 0}>
								{picked} / {group.items.length} {m.pickup_count_picked()}
							</span>
						{/if}
						{#if !done}
							{@const key = group.key}
							<span class="text-sm text-neutral-600">{m.pickup_btn_pick()} {group.items.length} st</span>
							<input
								type="number"
								min="0"
								value={quantityInputs[key] ?? group.items.length}
								oninput={(e) => quantityInputs[key] = parseInt(e.currentTarget.value) || 0}
								class="w-16 text-center border rounded px-2 py-1 text-sm"
							/>
							<button
								onclick={() => markQuantityGroup(group, quantityInputs[group.key] ?? group.items.length)}
								class="text-xs bg-green-700 text-white px-3 py-1 rounded"
							>{m.btn_confirm()}</button>
						{:else if !anyPickedUp}
							<span class="text-sm font-medium text-orange-800">
								{picked} / {group.items.length} {m.pickup_count_picked()}
							</span>
							<button onclick={() => resetQuantityGroup(group)} class="text-xs text-neutral-400 hover:text-neutral-600">{m.btn_undo()}</button>
						{:else}
							<button onclick={() => resetQuantityGroup(group)} class="text-xs text-neutral-400 hover:text-neutral-600">{m.btn_undo()}</button>
						{/if}
						<button
							onclick={() => { issueSheetArticle = { id: rep.article_id, name: group.commercialName, isQuantityTracked: true, groupTotal: group.items.length }; pendingSwapArticleId = null; }}
							class="text-xs bg-orange-600 text-white px-2 py-1 rounded"
						>{m.report_issue_title()}</button>
					{/if}
				</div>
			</div>
			{#if expanded}
				{#if isUsableGroup}
					<div class="px-4 py-2 border-t space-y-2 text-xs text-neutral-600">
						{#if issuesByArticle[rep?.article_id ?? '']}
							{@const openIssues = issuesByArticle[rep?.article_id ?? '']}
							{#if openIssues.length > 0}
								{@const issue = openIssues[0]}
								<div>
									<div class="flex items-center gap-2 mb-1">
										<span class="font-medium text-neutral-700">{issue.title}</span>
										{#if issue.severity === 'unusable'}
											<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded">{m.issue_severity_unusable()}</span>
										{:else if issue.severity === 'usable'}
											<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded">{m.issue_severity_usable()}</span>
										{:else if issue.severity === 'missing'}
											<span class="text-xs bg-challengerpink-100 text-challengerpink-700 px-1.5 py-0.5 rounded">{m.issue_severity_missing()}</span>
										{/if}
									</div>
									{#if issue.description}
										<p class="text-neutral-600">{issue.description}</p>
									{/if}
								</div>
							{:else}
								<p class="text-neutral-400">Inga öppna felrapporter hittades.</p>
							{/if}
						{:else}
							<p class="text-neutral-400">{m.btn_loading()}</p>
						{/if}
					</div>
				{:else}
					{@render infoBlock(qImageIds, group.commercialName, rep?.location_id ?? '', rep?.article_description ?? '', rep?.article_instructions ?? '')}
				{/if}
			{/if}
		</div>
	{/each}

	<!-- Individually tracked items -->
	{#each trackedGroups as tGroup}
		{@const tKey = tGroup.commercialName}
		{@const expandable = hasExpandable(tGroup.imageIds, tGroup.description, tGroup.instructions)}
		{@const expanded = expandedGroups.has(tKey)}
		{#each tGroup.items as item}
		{@const reported = isReported(item.article_status)}
		{@const unusable = isUnusable(item.article_status)}
		<div class="border rounded"
			class:bg-green-50={item.pickup_status === 'picked_up' || item.pickup_status === 'swapped'}
			class:bg-red-50={!item.pickup_status && unusable}
			class:bg-orange-50={!item.pickup_status && !unusable && reported}
		>
			<div class="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
				<button type="button" onclick={() => expandable && toggleExpand(tKey)} class="flex-1 min-w-[8rem] text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
					<div class="font-medium text-sm">
						{item.common_name}
						{#if expandable && tGroup.items[0] === item}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}
						{#if unusable}
							<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded ml-1">{m.report_issue_unavailable()}</span>
						{:else if item.article_status === 'reported_usable'}
							<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">{m.availability_reported()}</span>
						{:else if item.article_status === 'incoming'}
							<span class="text-xs bg-blue-50 text-blue-700 border border-blue-200 px-1.5 py-0.5 rounded ml-1">{m.pickup_incoming_badge()}{#if item.article_expected_available_date} — {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
						{:else if item.article_status === 'under_repair'}
							<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded ml-1">{m.article_status_under_repair()}{#if item.article_expected_available_date} — {m.pickup_incoming_ready()} {new Date(item.article_expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
						{/if}
					</div>
					<div class="text-xs text-neutral-500">{item.location_name}{item.place ? ` · ${item.place}` : ''}</div>
				</button>

				<div class="flex flex-wrap items-center gap-1">
					{#if item.pickup_status}
						<span class="text-xs px-2 py-0.5 rounded bg-green-100 text-green-800">
							{pickupLabels[item.pickup_status] ?? item.pickup_status}
						</span>
						<button onclick={() => markPickup(item.id, '')} class="text-xs text-neutral-400 hover:text-neutral-600 ml-1">{m.btn_undo()}</button>
					{:else if swappingItemId !== item.id}
						{#if unusable}
							<button onclick={() => startSwap(item)} class="text-xs bg-blue-700 text-white px-2 py-1 rounded">{m.pickup_btn_swap()}</button>
							<button onclick={() => markPickup(item.id, 'picked_up')} class="text-xs text-neutral-500 underline px-1">{m.pickup_btn_pick_anyway()}</button>
						{:else if reported}
							<button onclick={() => markPickup(item.id, 'picked_up')} class="text-xs bg-green-700 text-white px-2 py-1 rounded">{m.pickup_btn_pick_anyway()}</button>
							<button onclick={() => startSwap(item)} class="text-xs text-blue-700 underline px-1">{m.pickup_btn_swap()}</button>
						{:else}
							<button onclick={() => markPickup(item.id, 'picked_up')} class="text-xs bg-green-700 text-white px-2 py-1 rounded">{m.pickup_status_picked_up()}</button>
							<button onclick={() => { issueSheetArticle = { id: item.article_id, name: item.common_name }; pendingSwapArticleId = item.article_id; }} class="text-xs bg-orange-600 text-white px-2 py-1 rounded">{m.report_issue_title()}</button>
							<button onclick={() => startSwap(item)} class="text-xs text-blue-700 underline px-1">{m.pickup_btn_swap()}</button>
						{/if}
					{/if}
				</div>
			</div>
			{#if expanded && tGroup.items[0] === item}
				{@render infoBlock(tGroup.imageIds, tGroup.commercialName, tGroup.locationId, tGroup.description, tGroup.instructions)}
			{/if}
		</div>

		{#if swappingItemId === item.id}
			<div class="border rounded p-3 bg-blue-50 text-sm">
				{#if swapCandidates.length <= 1 && swapCandidates[0]?.is_current}
					<p class="text-neutral-600 mb-2">{m.pickup_no_replacements()}</p>
				{:else}
					<p class="mb-2">{m.pickup_choose_replacement()} <strong>{item.common_name}</strong>:</p>
				{/if}
				<div class="space-y-1 mb-2">
					{#each swapCandidates as candidate}
						{@const candidateUnusable = isUnusable(candidate.status)}
						<label class="flex items-center gap-2" class:opacity-50={candidateUnusable}>
							<input
								type="radio"
								name="swap-{item.id}"
								value={candidate.id}
								bind:group={selectedSwapArticle}
								disabled={candidateUnusable}
							/>
							<span>
								{candidate.common_name}
								{#if candidate.is_current}
									<span class="text-xs text-neutral-400">({m.pickup_original_badge()})</span>
								{/if}
							</span>
							{#if candidateUnusable}
								<span class="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded">{m.report_issue_unavailable()}</span>
							{:else if candidate.status === 'reported_usable'}
								<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded">{m.availability_reported()}</span>
							{:else if candidate.status === 'incoming'}
								<span class="text-xs bg-blue-50 text-blue-700 px-1.5 py-0.5 rounded">{m.pickup_incoming_badge()}{#if candidate.expected_available_date} — {new Date(candidate.expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
							{:else if candidate.status === 'under_repair'}
								<span class="text-xs bg-neutral-100 text-neutral-700 px-1.5 py-0.5 rounded">{m.article_status_under_repair()}{#if candidate.expected_available_date} — {m.pickup_incoming_ready()} {new Date(candidate.expected_available_date).toLocaleDateString('sv', { day: 'numeric', month: 'short' })}{/if}</span>
							{/if}
							<span class="text-xs text-neutral-500">{candidate.location_name}{candidate.place ? ` · ${candidate.place}` : ''}</span>
						</label>
					{/each}
				</div>
				<div class="flex gap-2">
					<button onclick={() => confirmSwap(item.id, item.article_id)} disabled={!selectedSwapArticle || loading} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">{m.btn_confirm()}</button>
					<button onclick={cancelSwap} class="text-xs text-neutral-600 underline">{m.btn_cancel()}</button>
				</div>
			</div>
		{/if}
		{/each}
	{/each}
</div>

{#if issueSheetArticle}
	<ReportIssueSheet
		articleId={issueSheetArticle.id}
		articleName={issueSheetArticle.name}
		open={true}
		bookingId={bookingId}
		isQuantityTracked={issueSheetArticle.isQuantityTracked ?? false}
		groupTotal={issueSheetArticle.groupTotal ?? 0}
		onReported={() => onReported(issueSheetArticle!.id)}
		onClose={() => { issueSheetArticle = null; pendingSwapArticleId = null; }}
	/>
{/if}
