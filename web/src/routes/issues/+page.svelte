<script lang="ts">
	import { createApiClient, type Article, type ArticleEvent } from '$lib/api/client';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));

	let serverArticles = $derived(data.articles);
	let localOverrides = $state<Map<string, Partial<Article>>>(new Map());
	let articles = $derived(
		serverArticles.map(a => localOverrides.has(a.id) ? { ...a, ...localOverrides.get(a.id) } : a)
	);
	let expandedId = $state<string | null>(null);
	const EVENT_LIMIT = 6;
	let events = $state<ArticleEvent[]>([]);
	let hasMoreEvents = $state(false);
	let loadingEvents = $state(false);
	let newStatus = $state('');
	let comment = $state('');
	let error = $state('');
	let message = $state('');

	const allFilterOptions = [
		{ value: 'reported_usable', label: 'Felrapporterad — användbar', color: 'bg-orange-500' },
		{ value: 'reported_unusable', label: 'Felrapporterad — ej användbar', color: 'bg-red-500' },
		{ value: 'under_repair', label: 'Under reparation', color: 'bg-blue-500', managerOnly: true },
		{ value: 'lost', label: 'Saknas', color: 'bg-challengerpink-500' },
		{ value: 'archived', label: 'Arkiverad', color: 'bg-neutral-400', managerOnly: true },
	];

	let filterOptions = $derived(isManager ? allFilterOptions : allFilterOptions.filter(o => !o.managerOnly));

	let selectedStatuses = $derived(new Set(data.filter.split(',')));
	let showMine = $derived(data.mine);

	const statusLabels: Record<string, string> = {
		ok: 'OK',
		reported_usable: 'Felrapporterad — användbar',
		incoming: 'Inkommande',
		reported_unusable: 'Felrapporterad — ej användbar',
		under_repair: 'Under reparation',
		lost: 'Saknas',
		archived: 'Arkiverad',
	};

	const statusColors: Record<string, string> = {
		reported_usable: 'bg-orange-100 text-orange-800',
		reported_unusable: 'bg-red-100 text-red-800',
		under_repair: 'bg-blue-100 text-blue-800',
		lost: 'bg-challengerpink-100 text-challengerpink-800',
		archived: 'bg-neutral-100 text-neutral-500',
	};

	const eventLabels: Record<string, string> = {
		issue_reported: 'Problem rapporterat',
		issue_resolved: 'Problem löst',
		status_change: 'Statusändring',
		returned: 'Återlämnad',
		booked: 'Bokad',
		picked_up: 'Uthämtad',
		note: 'Anteckning'
	};

	const metaStatusLabels: Record<string, string> = {
		ok: 'OK',
		reported_usable: 'Felrapporterad — användbar',
		reported_unusable: 'Felrapporterad — ej användbar',
		under_repair: 'Under reparation',
		lost: 'Saknas',
		archived: 'Arkiverad',
	};

	function formatEventMeta(event: ArticleEvent): string {
		const m = event.metadata ?? {};
		const parts: string[] = [];
		if (m.severity === 'usable') parts.push('användbar');
		if (m.severity === 'unusable') parts.push('ej användbar');
		if (m.reason === 'lost' || m.reason === 'missing_at_pickup') parts.push('saknas');
		if (m.new_status && m.old_status) {
			parts.push(`${metaStatusLabels[m.old_status] ?? m.old_status} → ${metaStatusLabels[m.new_status] ?? m.new_status}`);
		} else if (m.new_status) {
			parts.push(`→ ${metaStatusLabels[m.new_status] ?? m.new_status}`);
		}
		return parts.join(' · ');
	}

	function toggleStatus(value: string) {
		const next = new Set(selectedStatuses);
		if (next.has(value)) {
			next.delete(value);
		} else {
			next.add(value);
		}
		navigate(next, showMine);
	}

	function navigate(statuses: Set<string>, mine: boolean) {
		const s = [...statuses].join(',');
		const params = new URLSearchParams();
		if (s) params.set('status', s);
		params.set('mine', mine ? 'true' : 'false');
		window.location.href = `/issues${params.toString() ? '?' + params : ''}`;
	}

	async function toggle(article: Article) {
		if (expandedId === article.id) { expandedId = null; return; }
		expandedId = article.id;
		newStatus = '';
		comment = '';
		error = '';
		loadingEvents = true;
		try {
			const result = await api.listArticleEvents(article.id, EVENT_LIMIT);
			events = result.events;
			hasMoreEvents = result.has_more;
		} catch { events = []; hasMoreEvents = false; }
		loadingEvents = false;
	}

	function flash(msg: string) { message = msg; setTimeout(() => message = '', 4000); }

	async function updateStatus(articleId: string) {
		if (!newStatus) return;
		error = '';
		try {
			const updated = await api.updateArticleStatus(articleId, { status: newStatus, comment: comment.trim() || undefined });
			localOverrides = new Map(localOverrides).set(articleId, { status: updated.status });
			const result = await api.listArticleEvents(articleId, EVENT_LIMIT);
			events = result.events;
			hasMoreEvents = result.has_more;
			flash(`Status ändrad till ${statusLabels[newStatus] ?? newStatus}`);
			newStatus = '';
			comment = '';
		} catch (e: any) { error = e.message; }
	}
</script>

<div class="max-w-4xl mx-auto p-4">
	<h1 class="text-heading-sm font-bold mb-4">Ärenden</h1>

	{#if message}
		<div class="bg-green-50 border border-green-200 rounded p-3 mb-4 text-green-800 text-sm">{message}</div>
	{/if}

	<div class="mb-4">
		<p class="text-xs text-neutral-600 mb-2">Visa artiklar med status:</p>
		<div class="flex flex-wrap gap-2">
			{#each filterOptions as opt}
				<button
					onclick={() => toggleStatus(opt.value)}
					class="text-xs px-3 py-1.5 rounded border flex items-center gap-1.5"
					class:opacity-40={!selectedStatuses.has(opt.value)}
				>
					<span class="w-2 h-2 rounded-full {opt.color}"></span>
					{opt.label}
				</button>
			{/each}
		</div>
		<label class="flex items-center gap-2 mt-2">
			<input type="checkbox" checked={showMine} onchange={() => navigate(selectedStatuses, !showMine)} />
			<span class="text-xs text-neutral-600">Visa bara mina ärenden</span>
		</label>
	</div>

	{#if articles.length === 0}
		<p class="text-neutral-500">Inga ärenden att visa.</p>
	{:else}
		<p class="text-sm text-neutral-500 mb-3">{articles.length} artiklar</p>
		<div class="space-y-2">
			{#each articles as article}
				<div class="border rounded">
					<button onclick={() => toggle(article)} class="w-full text-left px-4 py-3 hover:bg-neutral-50">
						<div class="flex items-center justify-between gap-3">
							<div>
								<span class="font-medium text-sm">{article.common_name}</span>
								<span class="text-xs text-neutral-500 ml-1">({article.commercial_name})</span>
							</div>
							<div class="flex items-center gap-2">
								<span class="text-xs px-2 py-0.5 rounded {statusColors[article.status] ?? 'bg-neutral-100'}">{statusLabels[article.status] ?? article.status}</span>
								<span class="text-xs text-neutral-400">{article.location_name}</span>
							</div>
						</div>
					</button>

					{#if expandedId === article.id}
						<div class="border-t px-4 py-3 space-y-3">
							{#if error}<p class="text-red-600 text-xs">{error}</p>{/if}

							<div class="space-y-2">
								<div class="flex items-end gap-2">
									<div>
										<span class="text-xs text-neutral-600 block mb-1">{isManager ? 'Ändra status' : 'Uppdatera rapport'}</span>
										<select bind:value={newStatus} class="border rounded px-2 py-1 text-sm" aria-label={isManager ? 'Ändra status' : 'Uppdatera rapport'}>
											<option value="">Välj...</option>
											{#if isManager}
												<option value="ok">OK (löst)</option>
												<option value="under_repair">Under reparation</option>
											{/if}
											<option value="reported_usable">Felrapporterad — användbar</option>
											<option value="reported_unusable">Felrapporterad — ej användbar</option>
											<option value="lost">Saknas</option>
											{#if isManager}
												<option value="archived">Arkiverad</option>
											{/if}
										</select>
									</div>
									<button onclick={() => updateStatus(article.id)} disabled={!newStatus} class="text-xs bg-blue-700 text-white px-3 py-1.5 rounded disabled:opacity-50">Uppdatera</button>
								</div>
								<textarea bind:value={comment} placeholder="Kommentar..." rows="2" class="block border rounded px-2 py-1 text-sm w-full"></textarea>
							</div>

							<hr class="border-neutral-200" />

							<div>
								<p class="text-xs font-medium text-neutral-600 mb-1">Historik</p>
								{#if loadingEvents}
									<p class="text-xs text-neutral-400">Laddar...</p>
								{:else if events.length === 0}
									<p class="text-xs text-neutral-400">Ingen historik</p>
								{:else}
									<div class="space-y-1">
										{#each events as event}
											<div class="text-xs">
												<div class="flex items-start gap-2">
													<span class="text-neutral-400 shrink-0">{new Date(event.created_at).toLocaleDateString('sv')}</span>
													<span class="font-medium">{eventLabels[event.event_type] ?? event.event_type}</span>
													{#if formatEventMeta(event)}<span class="text-neutral-500">{formatEventMeta(event)}</span>{/if}
													<span class="text-neutral-400 ml-auto shrink-0">{event.actor_name}</span>
												</div>
												{#if event.description}
													<p class="text-neutral-600 ml-[4.5rem] mt-0.5">{event.description}</p>
												{/if}
											</div>
										{/each}
									</div>
									{#if hasMoreEvents}
										<button
											class="text-xs text-blue-600 hover:text-blue-800 mt-2 cursor-pointer"
											onclick={async () => {
												const result = await api.listArticleEvents(article.id);
												events = result.events;
												hasMoreEvents = false;
											}}
										>
											Visa alla händelser
										</button>
									{/if}
								{/if}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
