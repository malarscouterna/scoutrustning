<script lang="ts">
	import { createApiClient, type Article, type ArticleEvent } from '$lib/api/client';
	import { statusLabels, statusColors, eventTypeLabels } from '$lib/labels';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import ImageAttachInput from '$lib/components/ImageAttachInput.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));

	let localOverrides = $state<Map<string, Partial<Article>>>(new Map());
	let myArticles = $derived(
		data.myArticles.map(a => localOverrides.has(a.id) ? { ...a, ...localOverrides.get(a.id) } : a)
	);
	let otherArticles = $derived(
		data.otherArticles.map(a => localOverrides.has(a.id) ? { ...a, ...localOverrides.get(a.id) } : a)
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
	let issueImageIds = $state<string[]>([]);
	let eventsContainerEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;

	onMount(() => {
		return () => { lightbox?.destroy(); lightbox = null; };
	});

	async function initLightbox() {
		if (lightbox || !browser || !eventsContainerEl) return;
		const { default: PhotoSwipeLightbox } = await import('photoswipe/lightbox');
		await import('photoswipe/style.css');
		const lb = new PhotoSwipeLightbox({
			gallery: eventsContainerEl!,
			children: 'a.pswp-issue-img',
			pswpModule: () => import('photoswipe'),
		});
		lb.init();
		lightbox = lb;
	}

	const allFilterOptions = [
		{ value: 'reported_usable', label: 'Felrapporterad - användbar', color: 'bg-orange-500' },
		{ value: 'reported_unusable', label: 'Felrapporterad - ej användbar', color: 'bg-red-500' },
		{ value: 'under_repair', label: 'Under reparation', color: 'bg-blue-500', managerOnly: true },
		{ value: 'lost', label: 'Saknas', color: 'bg-challengerpink-500' },
		{ value: 'archived', label: 'Arkiverad', color: 'bg-neutral-400', managerOnly: true },
	];

	let filterOptions = $derived(isManager ? allFilterOptions : allFilterOptions.filter(o => !o.managerOnly));

	let selectedStatuses = $derived(new Set(data.filter.split(',')));

	function formatEventMeta(event: ArticleEvent): string {
		const m = event.metadata ?? {};
		const parts: string[] = [];
		if (m.severity === 'usable') parts.push('användbar');
		if (m.severity === 'unusable') parts.push('ej användbar');
		if (m.reason === 'lost' || m.reason === 'missing_at_pickup') parts.push('saknas');
		if (m.new_status && m.old_status) {
			parts.push(`${statusLabels[m.old_status] ?? m.old_status} → ${statusLabels[m.new_status] ?? m.new_status}`);
		} else if (m.new_status) {
			parts.push(`→ ${statusLabels[m.new_status] ?? m.new_status}`);
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
		navigate(next, data.showResolved);
	}

	function navigate(statuses: Set<string>, resolved: boolean) {
		const s = [...statuses].join(',');
		const params = new URLSearchParams();
		if (s) params.set('status', s);
		if (resolved) params.set('resolved', 'true');
		window.location.href = `/issues${params.toString() ? '?' + params : ''}`;
	}

	async function toggle(article: Article) {
		if (expandedId === article.id) { expandedId = null; return; }
		expandedId = article.id;
		newStatus = '';
		comment = '';
		error = '';
		issueImageIds = [];
		lightbox?.destroy(); lightbox = null;
		loadingEvents = true;
		try {
			const result = await api.listArticleEvents(article.id, EVENT_LIMIT);
			events = result.events;
			hasMoreEvents = result.has_more;
		} catch { events = []; hasMoreEvents = false; }
		loadingEvents = false;
		if (events.some(e => Array.isArray(e.metadata?.image_ids) && e.metadata.image_ids.length > 0)) {
			setTimeout(() => initLightbox(), 0);
		}
	}

	function flash(msg: string) { message = msg; setTimeout(() => message = '', 4000); }

	async function updateStatus(articleId: string) {
		if (!newStatus) return;
		error = '';
		try {
			const updated = await api.updateArticleStatus(articleId, { status: newStatus, comment: comment.trim() || undefined, image_ids: issueImageIds.length ? issueImageIds : undefined });
			localOverrides = new Map(localOverrides).set(articleId, { status: updated.status });
			const result = await api.listArticleEvents(articleId, EVENT_LIMIT);
			events = result.events;
			hasMoreEvents = result.has_more;
			flash(`Status ändrad till ${statusLabels[newStatus] ?? newStatus}`);
			newStatus = '';
			comment = '';
			issueImageIds = [];
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
			<input type="checkbox" checked={data.showResolved} onchange={() => navigate(selectedStatuses, !data.showResolved)} />
			<span class="text-xs text-neutral-600">Visa avslutade</span>
		</label>
	</div>

	{#snippet articleList(articles: Article[])}
		{#if articles.length === 0}
			<p class="text-sm text-neutral-500">Inga ärenden att visa.</p>
		{:else}
			<div class="space-y-2">
				{#each articles as article}
					<div class="border rounded">
						<button onclick={() => toggle(article)} class="w-full text-left px-4 py-3 hover:bg-neutral-50">
							<div class="flex flex-wrap items-center justify-between gap-2">
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
												<option value="reported_usable">Felrapporterad - användbar</option>
												<option value="reported_unusable">Felrapporterad - ej användbar</option>
												<option value="lost">Saknas</option>
												{#if isManager}
													<option value="archived">Arkiverad</option>
												{/if}
											</select>
										</div>
										<button onclick={() => updateStatus(article.id)} disabled={!newStatus} class="text-xs bg-blue-700 text-white px-3 py-1.5 rounded disabled:opacity-50">Uppdatera</button>
									</div>
									<textarea bind:value={comment} placeholder="Kommentar..." rows="2" class="block border rounded px-2 py-1 text-sm w-full"></textarea>
									<ImageAttachInput bind:imageIds={issueImageIds} />
								</div>

								<hr class="border-neutral-200" />

								<div>
									<p class="text-xs font-medium text-neutral-600 mb-1">Historik</p>
									{#if loadingEvents}
										<p class="text-xs text-neutral-400">Laddar...</p>
									{:else if events.length === 0}
										<p class="text-xs text-neutral-400">Ingen historik</p>
									{:else}
										<div bind:this={eventsContainerEl} class="space-y-1">
											{#each events as event}
												<div class="text-xs">
													<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
														<span class="text-neutral-400 shrink-0">{new Date(event.created_at).toLocaleDateString('sv')}</span>
														<span class="font-medium">{eventTypeLabels[event.event_type] ?? event.event_type}</span>
														{#if formatEventMeta(event)}<span class="text-neutral-500">{formatEventMeta(event)}</span>{/if}
														<span class="text-neutral-400 shrink-0">{event.actor_name}</span>
													</div>
													{#if event.description || (Array.isArray(event.metadata?.image_ids) && event.metadata.image_ids.length > 0)}
														<div class="flex flex-wrap items-start gap-2 mt-0.5 pl-0.5">
															{#if Array.isArray(event.metadata?.image_ids) && event.metadata.image_ids.length > 0}
																{#each event.metadata.image_ids as imgId}
																	<a href="/api/v0/images/{imgId}.webp" data-pswp-width="1920" data-pswp-height="1440" class="pswp-issue-img block cursor-zoom-in shrink-0">
																		<img src="/api/v0/images/{imgId}_thumb.webp" alt="" class="h-40 rounded object-contain" />
																	</a>
																{/each}
															{/if}
															{#if event.description}
																<p class="text-neutral-600 min-w-[10rem] flex-1">{event.description}</p>
															{/if}
														</div>
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
													lightbox?.destroy(); lightbox = null;
													setTimeout(() => initLightbox(), 0);
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
	{/snippet}

	{#if myArticles.length > 0}
		<section class="mb-6">
			<h2 class="font-semibold text-sm text-neutral-700 mb-2">Mina ärenden</h2>
			{@render articleList(myArticles)}
		</section>
	{/if}

	{#if otherArticles.length > 0}
		<section>
			<h2 class="font-semibold text-sm text-neutral-700 mb-2">Övriga ärenden</h2>
			{@render articleList(otherArticles)}
		</section>
	{/if}

	{#if myArticles.length === 0 && otherArticles.length === 0}
		<p class="text-neutral-500">Inga ärenden att visa.</p>
	{/if}
</div>
