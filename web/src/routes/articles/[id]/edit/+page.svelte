<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { createApiClient } from '$lib/api/client';
	import ArticleForm from '$lib/components/ArticleForm.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let isGroupEdit = $derived($page.url.searchParams.get('group') === 'true');
	let error = $state('');
	let saving = $state(false);

	async function handleSubmit(articles: Record<string, unknown>[]) {
		saving = true;
		error = '';
		try {
			await api.updateArticle(data.article.id, articles[0], isGroupEdit);
			goto('/browse');
		} catch (e: any) {
			error = e.message;
		}
		saving = false;
	}

	async function handleDelete() {
		if (!confirm('Är du säker? Artikeln tas bort permanent.')) return;
		try {
			await api.deleteArticle(data.article.id);
			goto('/browse');
		} catch (e: any) {
			error = e.message;
		}
	}
	async function handleCountChange(newCount: number) {
		await api.updateGroupCount({
			commercial_name: data.article.commercial_name,
			location_id: data.article.location_id,
			new_count: newCount
		});
	}
</script>

<div class="max-w-2xl mx-auto p-4">
	<a href="/browse" class="text-sm text-blue-700 underline">← Utrustning</a>
	<h1 class="text-heading-sm font-bold mt-2 mb-4">
		Redigera {isGroupEdit ? data.article.commercial_name || data.article.common_name : data.article.common_name}
	</h1>

	<ArticleForm
		mode="edit"
		locations={data.locations}
		categories={data.categories}
		initial={data.article}
		isManager={true}
		individuallyTrackedEdit={data.article.individually_tracked && !isGroupEdit}
		quantityTrackedEdit={isGroupEdit}
		groupCount={data.groupCount}
		onCountChange={handleCountChange}
		submitLabel="Spara"
		onSubmit={handleSubmit}
		onCancel={() => goto('/browse')}
		{error}
		{saving}
	/>

	{#if data.article.status === 'archived' && !isGroupEdit}
		<div class="mt-8 pt-4 border-t">
			<button onclick={handleDelete} class="text-sm text-red-600 underline">Ta bort artikel permanent</button>
		</div>
	{/if}
</div>
