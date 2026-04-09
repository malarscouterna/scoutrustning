<script lang="ts">
	import { createApiClient, type ArticleEvent } from '$lib/api/client';
	import { hasRole } from '$lib/user';
	import { page } from '$app/stores';
	import ReportIssueForm from '$lib/components/ReportIssueForm.svelte';
	import ArticleEventHistory from '$lib/components/ArticleEventHistory.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let isManager = $derived(hasRole($page.data.user, 'equipment_manager'));
	let article = $state<typeof data.article>(undefined!);

	$effect(() => {
		article = data.article;
	});
	let reporting = $state(false);
	let message = $state('');

	const statusLabels: Record<string, string> = {
		ok: 'OK',
		reported_usable: 'Felrapporterad — användbar',
		incoming: 'Inkommande',
		reported_unusable: 'Felrapporterad — ej användbar',
		under_repair: 'Under reparation',
		lost: 'Saknas',
		archived: 'Arkiverad'
	};

	const statusColors: Record<string, string> = {
		ok: 'bg-green-100 text-green-800',
		reported_usable: 'bg-orange-100 text-orange-800',
		incoming: 'bg-blue-50 text-blue-700',
		reported_unusable: 'bg-red-100 text-red-800',
		under_repair: 'bg-neutral-100 text-neutral-700',
		lost: 'bg-challengerpink-100 text-challengerpink-800',
		archived: 'bg-neutral-100 text-neutral-500'
	};

	const approvalLabels: Record<string, string> = {
		none: 'Ingen',
		low: 'Låg',
		high: 'Hög'
	};

	function handleIssueReported(newStatus: string) {
		article = { ...article, status: newStatus };
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
		<div class="flex flex-wrap items-center gap-3 mb-2">
			<h1 class="text-heading-sm font-bold">{article.common_name}</h1>
			<span class="text-sm px-2 py-0.5 rounded {statusColors[article.status] ?? 'bg-neutral-100'}">
				{statusLabels[article.status] ?? article.status}
			</span>
		</div>

		{#if article.commercial_name}
			<p class="text-neutral-600 mb-1">{article.commercial_name}</p>
		{/if}

		<div class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-neutral-500 mb-4">
			<span>{article.category_name}</span>
			<span>{article.location_name}</span>
			{#if article.place}
				<span>{article.place}</span>
			{/if}
		</div>

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
			{#if article.purchase_date}
				<span>Inköpt: {article.purchase_date}</span>
			{/if}
			{#if article.purchase_price}
				<span>{article.purchase_price} kr</span>
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
				<a href="/articles/{article.id}/edit" class="text-sm text-blue-700 underline">Redigera</a>
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

	<h2 class="font-medium mb-2">Historik</h2>
	<ArticleEventHistory articleId={article.id} />
</div>
