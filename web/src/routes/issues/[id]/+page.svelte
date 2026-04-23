<script lang="ts">
	import { createApiClient, type IssueDetail, type GroupMember } from '$lib/api/client';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import { browser } from '$app/environment';
	import { onMount, untrack } from 'svelte';
	import ImageAttachInput from '$lib/components/ImageAttachInput.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';
	import type { PageData } from './$types';
	import { msg } from '$lib/msg';
	import { issueStatusColors, issueSeverityColors } from '$lib/styles';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	// untrack: intentionally capture initial value only; issue is updated manually via API responses
	let issue = $state<IssueDetail>(untrack(() => data.issue));

	let commentText = $state('');
	let commentImageIds = $state<string[]>([]);
	let submittingComment = $state(false);
	let commentError = $state('');

	let statusAction = $state('');
	let statusComment = $state('');
	let updatingStatus = $state(false);
	let statusError = $state('');

	let eventsEl = $state<HTMLElement | null>(null);
	let lightbox: any = null;

	let assignPickerOpen = $state(false);
	let assignMembers = $state<GroupMember[]>([]);
	let assignSearch = $state('');
	let assignLoading = $state(false);
	let assignError = $state('');

	let assignFiltered = $derived(
		assignMembers.filter(
			(m) =>
				!issue.assignees.some((a) => a.user_id === m.id) &&
				m.name.toLowerCase().includes(assignSearch.toLowerCase())
		)
	);

	async function openAssignPicker() {
		assignPickerOpen = true;
		assignSearch = '';
		if (assignMembers.length > 0) return;
		assignLoading = true;
		try {
			assignMembers = await api.listGroupMembers('trusted,manager');
		} catch {
			assignError = 'Kunde inte ladda användare';
		} finally {
			assignLoading = false;
		}
	}

	async function addAssignee(userId: string) {
		try {
			await api.addIssueAssignee(issue.id, userId);
			issue = await api.getIssue(issue.id);
		} catch {
			// assignment failed silently; UI stays open so user can retry
		}
	}

	async function removeAssignee(userId: string) {
		try {
			await api.removeIssueAssignee(issue.id, userId);
			issue = await api.getIssue(issue.id);
		} catch {
			// removal failed silently
		}
	}

	onMount(() => {
		return () => { lightbox?.destroy(); lightbox = null; };
	});

	$effect(() => {
		if (browser && eventsEl && issue.events.some(e => Array.isArray(e.metadata?.image_ids) && e.metadata.image_ids.length > 0)) {
			setTimeout(() => initLightbox(), 0);
		}
	});

	async function initLightbox() {
		if (lightbox || !browser || !eventsEl) return;
		const { default: PhotoSwipeLightbox } = await import('photoswipe/lightbox');
		await import('photoswipe/style.css');
		const lb = new PhotoSwipeLightbox({
			gallery: eventsEl!,
			children: 'a.pswp-issue-img',
			pswpModule: () => import('photoswipe'),
		});
		lb.init();
		lightbox = lb;
	}

	async function submitComment() {
		if (!commentText.trim()) return;
		commentError = '';
		submittingComment = true;
		try {
			issue = await api.addIssueComment(issue.id, {
				description: commentText.trim(),
				...(commentImageIds.length ? { image_ids: commentImageIds } : {})
			});
			commentText = '';
			commentImageIds = [];
			lightbox?.destroy(); lightbox = null;
		} catch (e: any) {
			commentError = translateError(e);
		}
		submittingComment = false;
	}

	async function changeStatus(newStatus: string) {
		statusError = '';
		updatingStatus = true;
		try {
			issue = await api.updateIssue(issue.id, {
				status: newStatus,
				...(statusComment.trim() ? { comment: statusComment.trim() } : {})
			});
			statusComment = '';
		} catch (e: any) {
			statusError = translateError(e);
		}
		updatingStatus = false;
		statusAction = '';
	}

	function formatDate(ts: string) {
		const d = new Date(ts);
		const now = new Date();
		const diff = now.getTime() - d.getTime();
		if (diff < 86400000 && d.getDate() === now.getDate()) return m.page_issue_today();
		if (diff < 172800000) return m.page_issue_yesterday();
		return d.toLocaleDateString('sv', { day: 'numeric', month: 'short', year: d.getFullYear() !== now.getFullYear() ? 'numeric' : undefined });
	}

	function formatEventVerb(event: IssueDetail['events'][0]): string {
		const meta = event.metadata ?? {};
		if (event.event_type === 'assignment' && meta.action) {
			return msg(`issue_event_type_assignment_${meta.action}`) ?? event.event_type;
		}
		return msg(`issue_event_type_${event.event_type}`) ?? event.event_type;
	}

	function formatEventMeta(event: IssueDetail['events'][0]): string {
		const meta = event.metadata ?? {};
		if (event.event_type === 'status_change' && meta.new_status) {
			return msg(`issue_status_${meta.new_status}`) ?? meta.new_status;
		}
		if (event.event_type === 'assignment' && meta.user_name) {
			return meta.user_name;
		}
		return '';
	}
</script>

<div class="max-w-2xl mx-auto p-4">
	<a href="/issues" class="text-sm text-neutral-500 hover:text-neutral-800 mb-4 inline-block">{m.page_issue_back_link()}</a>

	<div class="flex flex-wrap items-start justify-between gap-2 mb-1">
		<h1 class="text-heading-sm font-bold">{issue.title}</h1>
		<span class="text-xs px-2 py-1 rounded font-medium {issueStatusColors[issue.status] ?? 'bg-neutral-100'}">
			{msg(`issue_status_${issue.status}`) ?? issue.status}
		</span>
	</div>

	<div class="flex flex-wrap items-center gap-2 text-sm text-neutral-500 mb-4">
		<span class="px-2 py-0.5 rounded text-xs font-medium {issueSeverityColors[issue.severity] ?? 'bg-neutral-100'}">
			{msg(`issue_severity_${issue.severity}`) ?? issue.severity}
		</span>
		<span>{m.page_issue_reported_by()} {issue.reporter.name}</span>
		<span>·</span>
		<span>{formatDate(issue.created_at)}</span>
		{#if issue.updated_at !== issue.created_at}
			<span>· {m.page_issue_updated()} {formatDate(issue.updated_at)}</span>
		{/if}
	</div>

	<!-- Articles -->
	{#if issue.articles.length > 0}
		{@const grouped = (() => {
			// For quantity-tracked: group by commercial_name+location, show count and link to first
			const qt = new Map<string, { article: typeof issue.articles[0]; count: number }>();
			const ind: typeof issue.articles = [];
			for (const a of issue.articles) {
				if (!a.individually_tracked) {
					const key = a.commercial_name + '|' + a.location_name;
					if (!qt.has(key)) qt.set(key, { article: a, count: 1 });
					else qt.get(key)!.count++;
				} else {
					ind.push(a);
				}
			}
			return { ind, qt: [...qt.values()] };
		})()}
		<div class="mb-4">
			<p class="text-xs font-medium text-neutral-500 uppercase tracking-wide mb-1">{m.page_issue_equipment_heading()}</p>
			<div class="space-y-1">
				{#each grouped.ind as article}
					<a href="/articles/{article.id}" class="flex items-center justify-between border rounded px-3 py-2 hover:bg-neutral-50 text-sm">
						<span>
							<span class="font-medium">{article.commercial_name}</span>
							{#if article.common_name}
								<span class="text-neutral-500"> – {article.common_name}</span>
							{/if}
							<span class="text-neutral-400 text-xs ml-1">({article.location_name})</span>
						</span>
						<span class="text-neutral-400">›</span>
					</a>
				{/each}
				{#each grouped.qt as { article, count }}
					<a href="/articles/{article.id}" class="flex items-center justify-between border rounded px-3 py-2 hover:bg-neutral-50 text-sm">
						<span>
							<span class="font-medium">{article.commercial_name}</span>
							{#if count > 1}
								<span class="text-neutral-500"> ({count} st)</span>
							{/if}
							<span class="text-neutral-400 text-xs ml-1">({article.location_name})</span>
						</span>
						<span class="text-neutral-400">›</span>
					</a>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Assignees -->
	{#if issue.assignees.length > 0 || isManager}
		<div class="mb-4">
			<p class="text-xs font-medium text-neutral-500 uppercase tracking-wide mb-1">{m.page_issue_assigned_heading()}</p>
			<div class="flex flex-wrap items-center gap-2">
				{#if issue.assignees.length === 0 && !isManager}
					<p class="text-sm text-neutral-400">{m.page_issue_not_assigned()}</p>
				{/if}
				{#each issue.assignees as assignee}
					{#if isManager}
						<span class="flex items-center gap-1 text-sm bg-neutral-100 rounded px-2 py-1">
							{assignee.user_name}
							<button
								onclick={() => removeAssignee(assignee.user_id)}
								class="text-neutral-400 hover:text-neutral-700 leading-none ml-0.5"
								aria-label="Ta bort {assignee.user_name}"
							>×</button>
						</span>
					{:else}
						<span class="text-sm bg-neutral-100 rounded px-2 py-1">{assignee.user_name}</span>
					{/if}
				{/each}
				{#if isManager}
					<div class="relative">
						<button
							onclick={openAssignPicker}
							class="text-sm border border-neutral-300 rounded px-2 py-1 hover:bg-neutral-50"
						>{m.page_issue_assign_btn()}</button>
						{#if assignPickerOpen}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div
								class="absolute left-0 top-full mt-1 z-20 bg-white border border-neutral-200 rounded shadow-md w-60"
								onkeydown={(e) => e.key === 'Escape' && (assignPickerOpen = false)}
							>
								<div class="p-2 border-b border-neutral-100">
									<!-- svelte-ignore a11y_autofocus -->
									<input
										type="text"
										bind:value={assignSearch}
										placeholder={m.page_issue_assign_search_placeholder()}
										class="w-full text-sm px-2 py-1 border rounded focus:outline-none focus:ring-1 focus:ring-blue-400"
										autofocus
									/>
								</div>
								<div class="max-h-48 overflow-y-auto">
									{#if assignLoading}
										<p class="text-sm text-neutral-400 px-3 py-2">{m.page_issue_assign_loading()}</p>
									{:else if assignFiltered.length === 0}
										<p class="text-sm text-neutral-400 px-3 py-2">{m.page_issue_assign_no_results()}</p>
									{:else}
										{#each assignFiltered as member}
											<button
												onclick={() => { addAssignee(member.id); assignPickerOpen = false; }}
												class="w-full text-left px-3 py-2 text-sm hover:bg-neutral-50 flex flex-col"
											>
												<span>{member.name}</span>
												<span class="text-xs text-neutral-400">{msg(`team_access_${member.access_level}`) ?? member.access_level}</span>
											</button>
										{/each}
									{/if}
								</div>
								<div class="p-2 border-t border-neutral-100">
									<button
										onclick={() => (assignPickerOpen = false)}
										class="text-xs text-neutral-400 hover:text-neutral-600 w-full text-right"
									>Stäng</button>
								</div>
							</div>
						{/if}
					</div>
				{/if}
			</div>
			{#if assignError}
				<p class="text-red-600 text-xs mt-1">{assignError}</p>
			{/if}
		</div>
	{/if}

	<hr class="border-neutral-200 mb-4" />

	<!-- Events -->
	<div class="mb-4">
		<p class="text-xs font-medium text-neutral-500 uppercase tracking-wide mb-3">{m.page_issue_events_heading()}</p>
		{#if issue.events.length === 0}
			<p class="text-sm text-neutral-400">{m.page_issue_events_empty()}</p>
		{:else}
			<div bind:this={eventsEl} class="space-y-3">
				{#each issue.events as event}
					<div class="flex gap-3 text-sm">
						<span class="text-neutral-400 text-xs shrink-0 pt-0.5 w-16 text-right">{formatDate(event.created_at)}</span>
						<div class="flex-1 min-w-0">
							<div class="flex flex-wrap items-baseline gap-x-1.5">
								<span class="font-medium">{event.actor_name}</span>
								{#if event.event_type !== 'comment'}
									<span class="text-neutral-500">{formatEventVerb(event)}</span>
									{#if formatEventMeta(event)}
										<span class="text-neutral-600 font-medium">{formatEventMeta(event)}</span>
									{/if}
								{/if}
							</div>
							{#if event.description}
								<p class="text-neutral-700 mt-0.5">{event.description}</p>
							{/if}
							{#if Array.isArray(event.metadata?.image_ids) && event.metadata.image_ids.length > 0}
								<div class="flex flex-wrap gap-2 mt-1">
									{#each event.metadata.image_ids as imgId}
										<a href="/api/v0/images/{imgId}.webp" data-pswp-width="1920" data-pswp-height="1440" class="pswp-issue-img block cursor-zoom-in shrink-0">
											<img src="/api/v0/images/{imgId}_thumb.webp" alt="" class="h-32 rounded object-contain" />
										</a>
									{/each}
								</div>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Add comment -->
	<div class="border rounded p-3 mb-4">
		{#if commentError}
			<p class="text-red-600 text-xs mb-2">{commentError}</p>
		{/if}
		<label for="comment-input" class="sr-only">{m.page_issue_comment_sr_label()}</label>
		<textarea
			id="comment-input"
			bind:value={commentText}
			rows="2"
			placeholder={m.page_issue_comment_placeholder()}
			class="w-full text-sm border-0 focus:outline-none resize-none"
		></textarea>
		<div class="flex items-center justify-between mt-2 gap-2">
			<ImageAttachInput bind:imageIds={commentImageIds} />
			<button
				onclick={submitComment}
				disabled={submittingComment || !commentText.trim()}
				class="text-sm bg-blue-700 text-white px-4 py-1.5 rounded disabled:opacity-50 shrink-0"
			>
				{submittingComment ? m.btn_submitting() : m.btn_submit()}
			</button>
		</div>
	</div>

	<!-- Manager status actions -->
	{#if isManager}
		<div class="space-y-2">
			{#if statusError}
				<p class="text-red-600 text-xs">{statusError}</p>
			{/if}
			<div>
				<label for="status-comment" class="text-xs font-medium text-neutral-500 block mb-1">{m.page_issue_status_comment_label()} <span class="text-neutral-400 font-normal">{m.optional()}</span></label>
				<textarea
					id="status-comment"
					bind:value={statusComment}
					rows="2"
					placeholder={m.page_issue_optional_comment_placeholder()}
					class="w-full border rounded px-3 py-2 text-sm"
				></textarea>
			</div>
			<div class="flex flex-wrap gap-2">
				{#if issue.status === 'open'}
					<button
						onclick={() => changeStatus('in_progress')}
						disabled={updatingStatus}
						class="text-sm border border-yellow-400 text-yellow-800 px-3 py-1.5 rounded hover:bg-yellow-50 disabled:opacity-50"
					>
						{m.page_issue_btn_in_progress()}
					</button>
				{/if}
				{#if issue.status === 'open' || issue.status === 'in_progress'}
					<button
						onclick={() => changeStatus('resolved')}
						disabled={updatingStatus}
						class="text-sm border border-green-500 text-green-800 px-3 py-1.5 rounded hover:bg-green-50 disabled:opacity-50"
					>
						{m.page_issue_btn_resolved()}
					</button>
					<button
						onclick={() => changeStatus('archived')}
						disabled={updatingStatus}
						class="text-sm border border-neutral-300 text-neutral-600 px-3 py-1.5 rounded hover:bg-neutral-50 disabled:opacity-50"
					>
						{m.page_issue_btn_archive()}
					</button>
				{/if}
				{#if issue.status === 'resolved' || issue.status === 'archived'}
					<button
						onclick={() => changeStatus('open')}
						disabled={updatingStatus}
						class="text-sm border border-blue-300 text-blue-700 px-3 py-1.5 rounded hover:bg-blue-50 disabled:opacity-50"
					>
						{m.page_issue_btn_reopen()}
					</button>
				{/if}
			</div>
		</div>
	{/if}
</div>
