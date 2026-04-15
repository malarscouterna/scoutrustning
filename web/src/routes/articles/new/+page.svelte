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
			for (const article of articles) {
				await api.createArticle(article);
			}
			goto('/browse');
		} catch (e: any) {
			error = e.message;
		}
		saving = false;
	}
</script>

<div class="max-w-2xl mx-auto p-4">
	<h1 class="text-heading-sm font-bold mt-2 mb-4">Ny artikel</h1>

	<ArticleForm
		mode="create"
		locations={data.locations}
		categories={data.categories}
		initial={data.prefill ?? undefined}
		isManager={true}
		submitLabel="Skapa"
		onSubmit={handleSubmit}
		onCancel={() => goto('/browse')}
		{error}
		{saving}
	/>
</div>
