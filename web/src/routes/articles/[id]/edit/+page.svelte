<script lang="ts">
	import { goto } from '$app/navigation';
	import { createApiClient } from '$lib/api/client';
	import ArticleForm from '$lib/components/ArticleForm.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();

	let error = $state('');
	let saving = $state(false);

	async function handleSubmit(articles: Record<string, unknown>[]) {
		saving = true;
		error = '';
		try {
			await api.updateArticle(data.article.id, articles[0]);
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
</script>

<div class="max-w-2xl mx-auto p-4">
	<a href="/browse" class="text-sm text-blue-700 underline">← Utrustning</a>
	<h1 class="text-heading-sm font-bold mt-2 mb-4">Redigera {data.article.common_name}</h1>

	<ArticleForm
		mode="edit"
		locations={data.locations}
		categories={data.categories}
		initial={data.article}
		isManager={true}
		submitLabel="Spara"
		onSubmit={handleSubmit}
		onCancel={() => goto('/browse')}
		{error}
		{saving}
	/>

	{#if data.article.status === 'archived'}
		<div class="mt-8 pt-4 border-t">
			<button onclick={handleDelete} class="text-sm text-red-600 underline">Ta bort artikel permanent</button>
		</div>
	{/if}
</div>
