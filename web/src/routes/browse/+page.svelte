<script lang="ts">
	import type { PageData } from './$types';
	import { createApiClient, type Article } from '$lib/api/client';
	import { statusLabels } from '$lib/labels';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import ReportIssueForm from '$lib/components/ReportIssueForm.svelte';
	import ArticleEventHistory from '$lib/components/ArticleEventHistory.svelte';
	import ImageViewer from '$lib/components/ImageViewer.svelte';

	let { data }: { data: PageData } = $props();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let managerMode = $state(false);
	let selectedArticles = $state<Set<string>>(new Set());
	let selectedGroups = $state<Set<string>>(new Set());
	let bulkStatus = $state('');
	let bulkLocationId = $state('');
	let bulkApproval = $state('');
	let bulkComment = $state('');
	let bulkLoading = $state(false);
	let bulkMessage = $state('');
	let bulkConflicts = $state<Array<{ article_id: string; article_name: string; booking_id: string; booking_dates: string; booking_unit: string }>>([]);

	let selectedCount = $derived(selectedArticles.size);
	let hasQuantityGroupSelected = $derived(
		[...selectedGroups].some(key => {
			const g = groups.find(gr => gr.key === key);
			return g && !g.individuallyTracked;
		})
	);

	function toggleSelectArticle(id: string) {
		const next = new Set(selectedArticles);
		if (next.has(id)) next.delete(id); else next.add(id);
		selectedArticles = next;
	}

	function toggleSelectGroup(group: ArticleGroup) {
		const ids = group.articles.map(a => a.id);
		const allSelected = ids.every(id => selectedArticles.has(id));
		const next = new Set(selectedArticles);
		const nextGroups = new Set(selectedGroups);
		for (const id of ids) {
			if (allSelected) next.delete(id); else next.add(id);
		}
		if (allSelected) nextGroups.delete(group.key); else nextGroups.add(group.key);
		selectedArticles = next;
		selectedGroups = nextGroups;
	}

	function isGroupSelected(group: ArticleGroup): boolean {
		return group.articles.length > 0 && group.articles.every(a => selectedArticles.has(a.id));
	}

	async function executeBulkAction() {
		if (selectedCount === 0) return;
		const action = bulkStatus || bulkLocationId || bulkApproval;
		if (!action) return;
		bulkLoading = true;
		bulkMessage = '';
		bulkConflicts = [];
		try {
			const payload: { article_ids: string[]; status?: string; location_id?: string; approval_level?: string; comment?: string } = {
				article_ids: [...selectedArticles]
			};
			if (bulkStatus) payload.status = bulkStatus;
			if (bulkLocationId) payload.location_id = bulkLocationId;
			if (bulkApproval) payload.approval_level = bulkApproval;
			if (bulkComment.trim()) payload.comment = bulkComment.trim();
			const result = await api.bulkUpdateArticles(payload);
			if (result.conflicts.length > 0) {
				bulkConflicts = result.conflicts;
				bulkMessage = `${result.conflicts.length} artiklar kunde inte arkiveras — aktiva bokningar.`;
			} else {
				bulkMessage = `${result.updated} artiklar uppdaterade.`;
				selectedArticles = new Set();
				selectedGroups = new Set();
				bulkStatus = '';
				bulkLocationId = '';
				bulkApproval = '';
				bulkComment = '';
				window.location.reload();
			}
		} catch (e: any) {
			bulkMessage = e.message ?? 'Något gick fel.';
		} finally {
			bulkLoading = false;
		}
	}

	let search = $state('');
	let selectedCategory = $state('');
	let selectedLocation = $state('');
	let showArchived = $state(false);

	$effect(() => {
		search = data.filters.search ?? '';
		selectedCategory = data.filters.category_id ?? '';
		selectedLocation = data.filters.location_id ?? '';
		showArchived = data.filters.status?.includes('archived') ?? false;
	});
	let expandedGroups = $state<Set<string>>(new Set());
	let reportingArticleId = $state<string | null>(null);
	let showHistoryFor = $state<string | null>(null);
	let showIssueHistoryFor = $state<string | null>(null);
	let issueHistory = $state<Map<string, { events: any[]; loading: boolean }>>(new Map());
	let reportedMessage = $state('');
	let latestComments = $state<Map<string, string>>(new Map());

	let expandedDescriptions = $state<Set<string>>(new Set());

	function toggleDescriptionExpand(key: string) {
		const next = new Set(expandedDescriptions);
		if (next.has(key)) next.delete(key); else next.add(key);
		expandedDescriptions = next;
	}

	const api = createApiClient();

	const statusOrder = ['ok', 'reported_usable', 'incoming', 'reported_unusable', 'under_repair', 'lost', 'archived'] as const;

	const bookableStatuses = new Set(['ok', 'reported_usable']);

	function isAvailable(a: Article): boolean {
		return bookableStatuses.has(a.status) && !a.current_booking_id;
	}

	function bookingLabel(a: Article): string | null {
		if (!a.current_booking_id) return null;
		const unit = a.current_booking_unit_name ?? 'Okänd';
		const endDate = a.current_booking_end_date ? formatDate(a.current_booking_end_date) : '?';
		if (a.current_booking_status === 'picked_up') return `Utlånad till ${unit}, tillbaka ${endDate}`;
		return `Reserverad för ${unit}, ${endDate}`;
	}

	function expectedDateLabel(a: Article): string | null {
		if (!a.expected_available_date) return null;
		if (a.status === 'incoming') return `beräknas levereras ${formatDate(a.expected_available_date)}`;
		if (a.status === 'under_repair') return `beräknas klar ${formatDate(a.expected_available_date)}`;
		return null;
	}

	function formatDate(iso: string): string {
		const d = new Date(iso);
		return d.toLocaleDateString('sv', { day: 'numeric', month: 'short' });
	}

	function sortArticles(items: Article[]): Article[] {
		return [...items].sort((a, b) => {
			const ai = statusOrder.indexOf(a.status as any);
			const bi = statusOrder.indexOf(b.status as any);
			if (ai !== bi) return ai - bi;
			// Within same status: available before booked
			const aBooked = a.current_booking_id ? 1 : 0;
			const bBooked = b.current_booking_id ? 1 : 0;
			return aBooked - bBooked;
		});
	}

	let articles: Article[] = $state([]);

	$effect(() => {
		articles = data.articles;
	});

	interface ArticleGroup {
		key: string;
		commercialName: string;
		categoryName: string;
		locationName: string;
		count: number;
		nonArchivedCount: number;
		articles: Article[];
		individuallyTracked: boolean;
		representativeId: string;
	}

	let groups = $derived.by(() => {
		const map = new Map<string, ArticleGroup>();
		for (const a of articles) {
			const key = `${a.commercial_name}||${a.location_name}`;
			const existing = map.get(key);
			if (existing) {
				existing.count++;
				if (a.status !== 'archived') existing.nonArchivedCount++;
				existing.articles.push(a);
			} else {
				map.set(key, {
					key,
					commercialName: a.commercial_name,
					categoryName: a.category_name,
					locationName: a.location_name,
					count: 1,
					nonArchivedCount: a.status !== 'archived' ? 1 : 0,
					articles: [a],
					individuallyTracked: a.individually_tracked,
					representativeId: a.id
				});
			}
		}
		return [...map.values()].sort((a, b) =>
			a.categoryName.localeCompare(b.categoryName, 'sv') ||
			a.commercialName.localeCompare(b.commercialName, 'sv')
		);
	});

	function toggleGroup(key: string) {
		if (expandedGroups.has(key)) {
			expandedGroups.delete(key);
		} else {
			expandedGroups.add(key);
			fetchLatestComments(key);
		}
		expandedGroups = new Set(expandedGroups);
	}

	async function fetchLatestComments(groupKey: string) {
		const group = groups.find(g => g.key === groupKey);
		if (!group) return;
		const needsFetch = group.articles.filter(a => a.status !== 'ok' && !latestComments.has(a.id));
		const results = await Promise.all(
			needsFetch.map(async (a) => {
				try {
					const { events } = await api.listArticleEvents(a.id);
					const issueEvent = events.find(e => e.event_type === 'issue_reported' || e.event_type === 'status_change');
					return [a.id, issueEvent?.description ?? ''] as const;
				} catch {
					return [a.id, ''] as const;
				}
			})
		);
		const next = new Map(latestComments);
		for (const [id, comment] of results) {
			if (comment) next.set(id, comment);
		}
		latestComments = next;
	}

	async function toggleIssueHistory(articleId: string) {
		if (showIssueHistoryFor === articleId) {
			showIssueHistoryFor = null;
			return;
		}
		showIssueHistoryFor = articleId;
		if (issueHistory.has(articleId)) return;
		const next = new Map(issueHistory);
		next.set(articleId, { events: [], loading: true });
		issueHistory = next;
		try {
			const { events } = await api.listArticleEvents(articleId);
				let filtered: typeof events;
			// Show events since last time status was set to ok (current issue cycle)
			filtered = [];
			for (const e of events) {
				const meta = e.metadata ?? {};
				if (meta.new_status === 'ok') break;
				filtered.push(e);
			}
			const updated = new Map(issueHistory);
			updated.set(articleId, { events: filtered, loading: false });
			issueHistory = updated;
		} catch {
			const updated = new Map(issueHistory);
			updated.set(articleId, { events: [], loading: false });
			issueHistory = updated;
		}
	}

	function applyFilters() {
		const params = new URLSearchParams();
		if (search) params.set('search', search);
		if (selectedCategory) params.set('category', selectedCategory);
		if (selectedLocation) params.set('location', selectedLocation);
		if (showArchived) params.set('status', 'ok,reported_usable,incoming,reported_unusable,under_repair,lost,archived');
		const qs = params.toString();
		window.location.href = `/browse${qs ? '?' + qs : ''}`;
	}

	function clearFilters() {
		search = '';
		selectedCategory = '';
		selectedLocation = '';
		window.location.href = '/browse';
	}

	function handleIssueReported(newStatus: string) {
		if (reportingArticleId) {
			articles = articles.map((a) => a.id === reportingArticleId ? { ...a, status: newStatus } : a);
		}
		reportingArticleId = null;
		reportedMessage = 'Problem rapporterat!';
		setTimeout(() => reportedMessage = '', 4000);
	}

	function statusBadgeClass(status: string): string {
		if (status === 'ok') return 'bg-green-100 text-green-800';
		if (status.startsWith('reported')) return 'bg-orange-100 text-orange-800';
		if (status === 'lost') return 'bg-challengerpink-100 text-challengerpink-800';
		if (status === 'archived') return 'bg-neutral-100 text-neutral-500';
		if (status === 'incoming') return 'bg-blue-50 text-blue-700 border border-blue-200';
		if (status === 'under_repair') return 'bg-neutral-100 text-neutral-700';
		return 'bg-neutral-100';
	}

	interface StateRow {
		key: string;
		status: string;
		count: number;
		bookingInfo: string | null;
		expectedDate: string | null;
		articleIds: string[];
	}

	function groupByState(items: Article[]): StateRow[] {
		const map = new Map<string, StateRow>();
		for (const a of items) {
			const booking = bookingLabel(a);
			const expected = expectedDateLabel(a);
			const key = `${a.status}||${booking ?? ''}||${expected ?? ''}`;
			const existing = map.get(key);
			if (existing) {
				existing.count++;
				existing.articleIds.push(a.id);
			} else {
				map.set(key, { key, status: a.status, count: 1, bookingInfo: booking, expectedDate: expected, articleIds: [a.id] });
			}
		}
		// Sort: status order, then non-booked before booked
		return [...map.values()].sort((a, b) => {
			const ai = statusOrder.indexOf(a.status as any);
			const bi = statusOrder.indexOf(b.status as any);
			if (ai !== bi) return ai - bi;
			const aBooked = a.bookingInfo ? 1 : 0;
			const bBooked = b.bookingInfo ? 1 : 0;
			return aBooked - bBooked;
		});
	}
</script>

<div class="max-w-4xl mx-auto p-4">
	<h1 class="text-heading-sm font-bold mb-4">Utrustning</h1>

	<div class="flex flex-wrap gap-2 mb-4">
		<input
			type="search"
			placeholder="Sök..."
			bind:value={search}
			onkeydown={(e) => e.key === 'Enter' && applyFilters()}
			class="border rounded px-3 py-2 flex-1 min-w-48"
		/>
		<select bind:value={selectedCategory} onchange={applyFilters} class="border rounded px-3 py-2">
			<option value="">Alla kategorier</option>
			{#each data.categories as cat}
				<option value={cat.id}>{cat.name}</option>
			{/each}
		</select>
		<select bind:value={selectedLocation} onchange={applyFilters} class="border rounded px-3 py-2">
			<option value="">Alla platser</option>
			{#each data.locations as loc}
				<option value={loc.id}>{loc.name}</option>
			{/each}
		</select>
		{#if search || selectedCategory || selectedLocation}
			<button onclick={clearFilters} class="text-sm underline px-2">Rensa</button>
		{/if}
	</div>

	<p class="text-sm text-neutral-600 mb-4">
		{articles.length} artiklar i {groups.length} grupper
	</p>

	{#if isManager}
		<div class="flex flex-wrap items-center gap-3 mb-4">
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={showArchived} onchange={applyFilters} />
				Visa arkiverade
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={managerMode} />
				Hanteringsläge
			</label>
			{#if managerMode}
				<a href="/articles/new" class="text-sm bg-blue-600 text-white px-3 py-1.5 rounded hover:bg-blue-700">+ Skapa artikel</a>
			{/if}
		</div>
		{#if managerMode && selectedCount > 0}
			{@const activeAction = bulkStatus ? 'status' : bulkLocationId ? 'location' : bulkApproval ? 'approval' : null}
			<div class="flex flex-wrap items-center gap-2 mb-4 p-3 bg-blue-50 border border-blue-200 rounded text-sm">
				<span class="font-medium">{selectedCount} markerade</span>
				{#if !activeAction || activeAction === 'status'}
					<select bind:value={bulkStatus} class="border rounded px-2 py-1 text-xs">
						<option value="">Ändra status...</option>
						<option value="ok">OK</option>
						<option value="under_repair">Under reparation</option>
						<option value="lost">Saknas</option>
						{#if !hasQuantityGroupSelected}
							<option value="archived">Arkivera</option>
						{/if}
					</select>
				{/if}
				{#if !activeAction || activeAction === 'location'}
					<select bind:value={bulkLocationId} class="border rounded px-2 py-1 text-xs">
						<option value="">Flytta till...</option>
						{#each data.locations as loc}
							<option value={loc.id}>{loc.name}</option>
						{/each}
					</select>
				{/if}
				{#if !activeAction || activeAction === 'approval'}
					<select bind:value={bulkApproval} class="border rounded px-2 py-1 text-xs">
						<option value="">Godkännande...</option>
						<option value="none">Ingen</option>
						<option value="low">Låg</option>
						<option value="high">Hög</option>
					</select>
				{/if}
				{#if activeAction}
					<input
						type="text"
						bind:value={bulkComment}
						placeholder="Kommentar..."
						class="border rounded px-2 py-1 text-xs flex-1 min-w-32"
					/>
				{/if}
				{#if activeAction}
					<button
						onclick={executeBulkAction}
						disabled={bulkLoading}
						class="text-xs bg-blue-600 text-white px-3 py-1 rounded hover:bg-blue-700 disabled:opacity-50"
					>
						{bulkLoading ? '...' : 'Utför'}
					</button>
					<button onclick={() => { bulkStatus = ''; bulkLocationId = ''; bulkApproval = ''; bulkComment = ''; }} class="text-xs text-neutral-500 underline">Ångra val</button>
				{/if}
				<button onclick={() => { selectedArticles = new Set(); selectedGroups = new Set(); bulkStatus = ''; bulkLocationId = ''; bulkApproval = ''; bulkComment = ''; }} class="text-xs text-neutral-500 underline">Avmarkera</button>
				{#if hasQuantityGroupSelected && !activeAction}
					<span class="text-xs text-neutral-500">Arkivering av antalsspårade grupper görs via antal-fältet</span>
				{/if}
			</div>
		{/if}
	{/if}

	{#if bulkMessage}
		<div class="bg-amber-50 border border-amber-200 rounded p-3 mb-4 text-amber-800 text-sm">
			{bulkMessage}
			{#if bulkConflicts.length > 0}
				<ul class="mt-1 list-disc list-inside">
					{#each bulkConflicts as c}
						<li>{c.article_name} — bokad av {c.booking_unit} ({c.booking_dates})</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}

	{#if reportedMessage}
		<div class="bg-green-50 border border-green-200 rounded p-3 mb-4 text-green-800 text-sm">{reportedMessage}</div>
	{/if}

	<div class="space-y-1">
		{#each groups as group (group.key)}
			{@const expanded = expandedGroups.has(group.key)}
			{@const availableCount = group.articles.filter(isAvailable).length}
			<div class="border rounded">
				<button
					onclick={() => toggleGroup(group.key)}
					class="w-full flex flex-wrap items-center justify-between gap-1 px-4 py-3 hover:bg-neutral-50 text-left"
				>
					<div class="flex items-center gap-2 min-w-0">
						{#if managerMode}
							<input type="checkbox" checked={isGroupSelected(group)} onclick={(e) => { e.stopPropagation(); toggleSelectGroup(group); }} class="shrink-0" />
						{/if}
						<span class="font-medium">{group.commercialName}</span>
						<span class="text-sm text-neutral-500 ml-2">{group.categoryName}</span>
					</div>
					<div class="flex items-center gap-2 text-sm text-neutral-600">
						<span>{group.locationName}</span>
						{#if group.individuallyTracked}
							<span class="bg-blue-600 text-white px-2 py-0.5 rounded">{availableCount}/{group.nonArchivedCount} st</span>
						{:else}
							<span class="bg-blue-100 text-blue-800 px-2 py-0.5 rounded">×{availableCount}/{group.nonArchivedCount}</span>
						{/if}
						<span class="text-xs">{expanded ? '▲' : '▼'}</span>
					</div>
				</button>
				{#if expanded}
					{@const rep = group.articles[0]}
					{@const hasImage = rep.image_ids?.length > 0}
					{@const hasTextInfo = !!(rep.description || rep.instructions || (isManager && rep.manager_notes))}
					<div class="border-t px-4 py-2 bg-neutral-50">
						{#if !managerMode && hasImage}
							<div class="mb-2">
								<ImageViewer imageIds={rep.image_ids} alt={rep.commercial_name || rep.common_name} commercialName={rep.commercial_name} locationId={rep.location_id} />
							</div>
						{/if}
						{#if group.individuallyTracked}
							{#if hasTextInfo}
								{@render inlineTextInfo(rep, group.key)}
							{/if}
							<div class="divide-y divide-neutral-200 text-sm">
								{#each sortArticles(group.articles) as article}
									<div class="py-2">
										<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
											{#if managerMode}
												<input type="checkbox" checked={selectedArticles.has(article.id)} onclick={() => toggleSelectArticle(article.id)} class="shrink-0" />
											{/if}
											<a href="/articles/{article.id}" class="inline-flex items-center gap-1 text-xs font-medium text-blue-700 border border-blue-200 bg-blue-50 rounded px-2 py-0.5 hover:bg-blue-100">{article.common_name} ›</a>
											<span class="text-xs text-neutral-500">{article.place || '—'}</span>
											{#if article.status !== 'ok' || article.current_booking_id}
												<button onclick={() => toggleIssueHistory(article.id)} class="inline-block px-2 py-0.5 rounded text-xs cursor-pointer {statusBadgeClass(article.status)}">
													{statusLabels[article.status] ?? article.status}
												</button>
											{:else}
												<span class="inline-block px-2 py-0.5 rounded text-xs {statusBadgeClass(article.status)}">
													{statusLabels[article.status] ?? article.status}
												</span>
											{/if}
											<span class="ml-auto flex gap-2 shrink-0">
												{#if isManager}
													<a href="/articles/{article.id}/edit" class="inline-flex items-center gap-1 text-xs text-neutral-600 border border-neutral-200 bg-neutral-50 rounded px-2 py-0.5 hover:bg-neutral-100">Redigera ›</a>
												{/if}
												<button onclick={() => reportingArticleId = reportingArticleId === article.id ? null : article.id} class="text-xs text-blue-700 underline">Rapportera</button>
												<button onclick={() => showHistoryFor = showHistoryFor === article.id ? null : article.id} class="text-xs text-neutral-500 underline">Historik</button>
											</span>
										</div>
										{#if bookingLabel(article)}
											<p class="text-xs text-purple-700 mt-0.5">{bookingLabel(article)}</p>
										{/if}
										{#if expectedDateLabel(article)}
											<p class="text-xs text-blue-600 mt-0.5">({expectedDateLabel(article)})</p>
										{/if}
										{#if latestComments.has(article.id) && showIssueHistoryFor !== article.id}
											<p class="text-xs text-neutral-500 mt-0.5 italic">“{latestComments.get(article.id)}”</p>
										{/if}
										{#if showIssueHistoryFor === article.id}
											{@const ih = issueHistory.get(article.id)}
											{#if ih?.loading}
												<p class="text-xs text-neutral-400 mt-1">Laddar...</p>
											{:else if ih && ih.events.length > 0}
												<div class="mt-1 space-y-1">
													{#each ih.events as event}
														<div class="text-xs text-neutral-600">
															<span class="text-neutral-400">{new Date(event.created_at).toLocaleDateString('sv')} — {event.actor_name}</span>
															{#if event.description}<p class="italic mt-0.5">“{event.description}”</p>{/if}
														</div>
													{/each}
												</div>
											{/if}
										{/if}
									</div>
									{#if reportingArticleId === article.id}
										<ReportIssueForm articleId={article.id} articleName={article.common_name} onReported={handleIssueReported} onCancel={() => reportingArticleId = null} />
									{/if}
									{#if showHistoryFor === article.id}
										<div class="py-2">
											<ArticleEventHistory articleId={article.id} />
										</div>
									{/if}
								{/each}
							</div>
						{:else}
							{@const rows = groupByState(group.articles)}
							<div class="space-y-1 py-1 text-sm">
								<div class="flex flex-wrap items-center gap-2">
									<a href="/articles/{group.representativeId}" class="inline-flex items-center gap-1 text-xs text-blue-700 border border-blue-200 bg-blue-50 rounded px-2 py-1 hover:bg-blue-100">Visa artikelsida ›</a>
									{#if isManager}
										<a href="/articles/{group.representativeId}/edit?group=true" class="inline-flex items-center gap-1 text-xs text-neutral-600 border border-neutral-200 bg-neutral-50 rounded px-2 py-1 hover:bg-neutral-100">Redigera ›</a>
									{/if}
								</div>
								{#if hasTextInfo}
									{@render inlineTextInfo(rep, group.key)}
								{/if}
								{#each rows as row}
									{@const comment = row.articleIds.map(id => latestComments.get(id)).find(c => c)}
									{@const representativeId = row.articleIds[0]}
									<div class="flex items-start gap-2">
										{#if row.status !== 'ok'}
											<button onclick={() => toggleIssueHistory(representativeId)} class="inline-block px-2 py-0.5 rounded text-xs cursor-pointer {statusBadgeClass(row.status)}">
												×{row.count} {statusLabels[row.status] ?? row.status}
											</button>
										{:else}
											<span class="inline-block px-2 py-0.5 rounded text-xs {statusBadgeClass(row.status)}">
												×{row.count} {statusLabels[row.status] ?? row.status}
											</span>
										{/if}
										<div class="min-w-0">
											{#if row.bookingInfo}
												<span class="text-xs text-purple-700">{row.bookingInfo}</span>
											{/if}
											{#if row.expectedDate}
												<span class="text-xs text-blue-600">({row.expectedDate})</span>
											{/if}
											{#if comment && showIssueHistoryFor !== representativeId}
												<p class="text-xs text-neutral-500 italic">“{comment}”</p>
											{/if}
											{#if showIssueHistoryFor === representativeId}
												{@const ih = issueHistory.get(representativeId)}
												{#if ih?.loading}
													<p class="text-xs text-neutral-400">Laddar...</p>
												{:else if ih && ih.events.length > 0}
													<div class="space-y-1">
														{#each ih.events as event}
															<div class="text-xs text-neutral-600">
																<span class="text-neutral-400">{new Date(event.created_at).toLocaleDateString('sv')} — {event.actor_name}</span>
																{#if event.description}<p class="italic mt-0.5">“{event.description}”</p>{/if}
															</div>
														{/each}
													</div>
												{/if}
											{/if}
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>

{#snippet inlineTextInfo(a: Article, key: string)}
	{@const isExpanded = expandedDescriptions.has(key)}
	<div class="mb-2 space-y-1 text-xs text-neutral-600">
		{#if a.description}
			<button type="button" onclick={() => toggleDescriptionExpand(key)} class="text-left cursor-pointer max-w-full">
				<p class:line-clamp-2={!isExpanded}>{a.description}</p>
			</button>
		{/if}
		{#if a.instructions}
			<button type="button" onclick={() => toggleDescriptionExpand(key)} class="text-left cursor-pointer max-w-full">
				<span class="font-medium text-neutral-500">Instruktioner:</span>
				<p class="mt-0.5" class:line-clamp-2={!isExpanded}>{a.instructions}</p>
			</button>
		{/if}
		{#if isManager && a.manager_notes}
			<div class="bg-amber-50 border border-amber-200 rounded p-2">
				<span class="font-medium text-amber-700">Intern anteckning:</span>
				<p class="mt-0.5 text-amber-900" class:line-clamp-2={!isExpanded}>{a.manager_notes}</p>
			</div>
		{/if}
	</div>
{/snippet}

{#snippet textInfoBlock(a: Article)}
	<div class="mb-2 space-y-2 text-xs text-neutral-600">
		{#if a.description}
			<div>
				<span class="font-medium text-neutral-500">Beskrivning:</span>
				<p class="mt-0.5">{a.description}</p>
			</div>
		{/if}
		{#if a.instructions}
			<div>
				<span class="font-medium text-neutral-500">Instruktioner:</span>
				<p class="mt-0.5">{a.instructions}</p>
			</div>
		{/if}
		{#if isManager && a.manager_notes}
			<div class="bg-amber-50 border border-amber-200 rounded p-2">
				<span class="font-medium text-amber-700">Intern anteckning:</span>
				<p class="mt-0.5 text-amber-900">{a.manager_notes}</p>
			</div>
		{/if}
	</div>
{/snippet}
