<script lang="ts">
	import { createApiClient, type ArticleEvent } from '$lib/api/client';
	import { statusLabels, statusColors, approvalLabels, eventTypeLabels, eventTypeColors } from '$lib/labels';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import ReportIssueForm from '$lib/components/ReportIssueForm.svelte';
	import ArticleEventHistory from '$lib/components/ArticleEventHistory.svelte';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let article = $derived(data.article);
	let groupArticles = $derived(data.groupArticles);
	let isQuantityTracked = $derived(!article.individually_tracked && groupArticles !== null);
	let statusOverride = $state<string | null>(null);
	let effectiveStatus = $derived(statusOverride ?? article.status);
	let reporting = $state(false);
	let message = $state('');

	// Group status summary for quantity tracked
	let statusSummary = $derived.by(() => {
		if (!groupArticles) return [];
		const counts = new Map<string, number>();
		for (const a of groupArticles) {
			counts.set(a.status, (counts.get(a.status) ?? 0) + 1);
		}
		const order = ['ok', 'reported_usable', 'incoming', 'reported_unusable', 'under_repair', 'lost', 'archived'];
		return order
			.filter(s => counts.has(s))
			.map(s => ({ status: s, count: counts.get(s)! }));
	});

	// Aggregated purchase info for quantity tracked
	let purchaseOverview = $derived.by(() => {
		if (!groupArticles) return null;
		const dates = groupArticles.filter(a => a.purchase_date).map(a => a.purchase_date!);
		const prices = groupArticles.filter(a => a.purchase_price).map(a => a.purchase_price!);
		if (dates.length === 0 && prices.length === 0) return null;
		const uniqueDates = [...new Set(dates)].sort();
		const uniquePrices = [...new Set(prices)].sort();
		return { dates: uniqueDates, prices: uniquePrices };
	});

	// Group events: loading state
	let groupEvents = $state<ArticleEvent[]>([]);
	let groupEventsHasMore = $state(false);
	let groupEventsLoading = $state(true);
	let groupEventsShowAll = $state(false);

	async function loadGroupEvents(limit?: number) {
		groupEventsLoading = true;
		try {
			const result = await api.listArticleGroupEvents(article.id, limit);
			groupEvents = result.events;
			groupEventsHasMore = result.has_more;
			groupEventsShowAll = !limit;
		} catch {
			// ignore
		} finally {
			groupEventsLoading = false;
		}
	}

	$effect(() => {
		if (isQuantityTracked) {
			loadGroupEvents(6);
		}
	});

	function formatEventMeta(event: ArticleEvent): string {
		const m = event.metadata ?? {};
		if (m.old_count && m.new_count) return `${m.old_count} → ${m.new_count}`;
		if (m.new_status && m.old_status) {
			return `${statusLabels[m.old_status] ?? m.old_status} → ${statusLabels[m.new_status] ?? m.new_status}`;
		}
		return '';
	}

	interface CollapsedEvent {
		event: ArticleEvent;
		count: number;
	}

	function collapseEvents(events: ArticleEvent[]): CollapsedEvent[] {
		const result: CollapsedEvent[] = [];
		for (const e of events) {
			const prev = result[result.length - 1];
			if (prev
				&& e.event_type !== 'note'
				&& prev.event.event_type === e.event_type
				&& prev.event.actor_name === e.actor_name
				&& formatEventMeta(prev.event) === formatEventMeta(e)
				&& Math.abs(new Date(prev.event.created_at).getTime() - new Date(e.created_at).getTime()) < 60_000
			) {
				prev.count++;
			} else {
				result.push({ event: e, count: 1 });
			}
		}
		return result;
	}

	let collapsedGroupEvents = $derived(collapseEvents(groupEvents));

	let noteText = $state('');
	let noteSaving = $state(false);
	let historyKey = $state(0);

	async function addNote() {
		if (!noteText.trim()) return;
		noteSaving = true;
		try {
			await api.addArticleNote(article.id, noteText.trim());
			noteText = '';
			if (isQuantityTracked) {
				await loadGroupEvents(groupEventsShowAll ? undefined : 6);
			} else {
				historyKey++;
			}
		} catch {
			// ignore
		} finally {
			noteSaving = false;
		}
	}

	function handleIssueReported(newStatus: string) {
		statusOverride = newStatus;
		reporting = false;
		message = 'Problem rapporterat!';
		setTimeout(() => message = '', 4000);
	}
</script>

<div class="max-w-2xl mx-auto p-4">
	<a href="/browse" class="text-sm text-blue-700 underline">← Utrustning</a>

	{#if message}
		<div class="bg-green-50 border border-green-200 rounded p-3 mt-4 text-green-800 text-sm">{message}</div>
	{/if}

	<div class="mt-4 mb-6">
		{#if article.image_path}
			<ImageViewer
				src="/api/v0/images/{article.image_path}.webp"
				thumbSrc="/api/v0/images/{article.image_path}_thumb.webp"
				alt={article.commercial_name || article.common_name}
				downloadId={article.image_path}
				class="block w-3/4 mb-4"
			/>
		{/if}
		<div class="flex flex-wrap items-center gap-3 mb-2">
			{#if isQuantityTracked}
				<h1 class="text-heading-sm font-bold">{article.commercial_name || article.common_name}</h1>
			{:else}
				<h1 class="text-heading-sm font-bold">{article.common_name}</h1>
				<span class="text-sm px-2 py-0.5 rounded {statusColors[effectiveStatus] ?? 'bg-neutral-100'}">
					{statusLabels[effectiveStatus] ?? effectiveStatus}
				</span>
			{/if}
		</div>

		{#if !isQuantityTracked && article.commercial_name}
			<p class="text-neutral-600 mb-1">{article.commercial_name}</p>
		{/if}

		<div class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-neutral-500 mb-4">
			<span>{article.category_name}</span>
			<span>{article.location_name}</span>
			{#if article.place}
				<span>{article.place}</span>
			{/if}
		</div>

		{#if isQuantityTracked && groupArticles}
			<div class="mb-4">
				<h2 class="text-sm font-medium text-neutral-600 mb-1">Status ({groupArticles.length} st)</h2>
				<div class="flex flex-wrap gap-2">
					{#each statusSummary as { status, count }}
						<span class="text-xs px-2 py-0.5 rounded {statusColors[status] ?? 'bg-neutral-100'}">
							{count} {statusLabels[status] ?? status}
						</span>
					{/each}
				</div>
			</div>
		{/if}

		{#if article.description}
			<div class="mb-4">
				<h2 class="text-sm font-medium text-neutral-600 mb-1">Beskrivning</h2>
				<p class="text-sm">{article.description}</p>
			</div>
		{/if}

		{#if article.instructions}
			<div class="mb-4">
				<h2 class="text-sm font-medium text-neutral-600 mb-1">Instruktioner</h2>
				<p class="text-sm">{article.instructions}</p>
			</div>
		{/if}

		<div class="flex flex-wrap gap-x-6 gap-y-2 text-sm text-neutral-500 mb-4">
			<span>Spårning: {article.individually_tracked ? 'Individuell' : 'Antal'}</span>
			<span>Godkännande: {approvalLabels[article.approval_level] ?? article.approval_level}</span>
			{#if isQuantityTracked && purchaseOverview}
				{#if purchaseOverview.dates.length > 0}
					<span>Inköpt: {purchaseOverview.dates.join(', ')}</span>
				{/if}
				{#if purchaseOverview.prices.length > 0}
					<span>{purchaseOverview.prices.join(', ')} kr</span>
				{/if}
			{:else}
				{#if article.purchase_date}
					<span>Inköpt: {article.purchase_date}</span>
				{/if}
				{#if article.purchase_price}
					<span>{article.purchase_price} kr</span>
				{/if}
			{/if}
		</div>

		{#if isManager && article.manager_notes}
			<div class="mb-4 bg-amber-50 border border-amber-200 rounded p-3">
				<h2 class="text-sm font-medium text-amber-800 mb-1">Interna anteckningar</h2>
				<p class="text-sm text-amber-900">{article.manager_notes}</p>
			</div>
		{/if}

		<div class="flex flex-wrap gap-2 mb-6">
			<button onclick={() => reporting = !reporting} class="text-sm text-blue-700 underline">
				{reporting ? 'Avbryt' : 'Rapportera problem'}
			</button>
			{#if isManager}
				<a href="/articles/{article.id}/edit{isQuantityTracked ? '?group=true' : ''}" class="inline-flex items-center gap-1 text-xs text-neutral-600 border border-neutral-200 bg-neutral-50 rounded px-2 py-1 hover:bg-neutral-100">Redigera ›</a>
			{/if}
		</div>

		{#if reporting}
			<div class="mb-6">
				<ReportIssueForm
					articleId={article.id}
					articleName={article.common_name}
					onReported={handleIssueReported}
					onCancel={() => reporting = false}
				/>
			</div>
		{/if}
	</div>

	<div class="flex gap-2 mb-4">
		<input
			type="text"
			bind:value={noteText}
			placeholder="Lägg till kommentar..."
			onkeydown={(e) => e.key === 'Enter' && addNote()}
			class="border rounded px-2 py-1.5 text-sm flex-1"
		/>
		<button onclick={addNote} disabled={noteSaving || !noteText.trim()} class="text-sm bg-blue-600 text-white px-3 py-1.5 rounded hover:bg-blue-700 disabled:opacity-50">
			{noteSaving ? '...' : 'Spara'}
		</button>
	</div>

	<h2 class="font-medium mb-2">Historik</h2>
	{#if isQuantityTracked}
		{#if groupEventsLoading}
			<p class="text-xs text-neutral-400 py-1">Laddar historik...</p>
		{:else if groupEvents.length === 0}
			<p class="text-xs text-neutral-400 py-1">Ingen historik</p>
		{:else}
			<div class="space-y-1.5 mt-1">
				{#each collapsedGroupEvents as { event, count }}
					<div class="text-xs">
						<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
							<span class="text-neutral-400 shrink-0">{new Date(event.created_at).toLocaleDateString('sv')}</span>
							<span class="font-medium {eventTypeColors[event.event_type] ?? 'text-neutral-700'}">{eventTypeLabels[event.event_type] ?? event.event_type}{#if count > 1} ×{count}{/if}</span>
							{#if formatEventMeta(event)}<span class="text-neutral-500">{formatEventMeta(event)}</span>{/if}
							<span class="text-neutral-400 shrink-0">{event.actor_name}</span>
						</div>
						{#if event.description}
							<p class="text-neutral-600 mt-0.5 pl-0.5">{event.description}</p>
						{/if}
					</div>
				{/each}
			</div>
			{#if groupEventsHasMore && !groupEventsShowAll}
				<button
					class="text-xs text-blue-600 hover:text-blue-800 mt-2 cursor-pointer"
					onclick={() => loadGroupEvents()}
				>
					Visa alla händelser
				</button>
			{/if}
		{/if}
	{:else}
		{#key historyKey}
			<ArticleEventHistory articleId={article.id} />
		{/key}
	{/if}
</div>
