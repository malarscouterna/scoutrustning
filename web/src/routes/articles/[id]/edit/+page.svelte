<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { createApiClient } from '$lib/api/client';
	import { msg } from '$lib/msg';
	import { articleStatusColors } from '$lib/styles';
	import ArticleForm from '$lib/components/ArticleForm.svelte';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import ImageUpload from '$lib/components/ImageUpload.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const api = createApiClient();
	let user = $derived(data.user);

	let isGroupEdit = $derived($page.url.searchParams.get('group') === 'true');
	let error = $state('');
	let saving = $state(false);
	let showItems = $state(false);
	let imageIds = $state<string[]>([]);

	$effect(() => {
		imageIds = data.article.image_ids ?? [];
	});

	function statusBadgeClass(status: string): string {
		return articleStatusColors[status] ?? 'bg-neutral-100';
	}

	async function handleSubmit(articles: Record<string, unknown>[]) {
		saving = true;
		error = '';
		try {
			const articleData = { ...articles[0] };
			const newCount = articleData._newCount as number | undefined;
			delete articleData._newCount;

			await api.updateArticle(data.article.id, articleData, isGroupEdit);

			if (newCount !== undefined) {
				await api.updateGroupCount({
					commercial_name: data.article.commercial_name,
					location_id: data.article.location_id,
					new_count: newCount
				});
			}

			goto('/browse');
		} catch (e: any) {
			error = e.message;
			saving = false;
		}
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
	<h1 class="text-heading-sm font-bold mt-2 mb-4">
		Redigera {isGroupEdit ? data.article.commercial_name || data.article.common_name : data.article.common_name}
	</h1>

	{#if data.article.commercial_name && data.article.location_id}
		<div class="mb-4">
			{#if imageIds.length > 0}
				<ImageViewer {imageIds} alt={data.article.commercial_name || data.article.common_name} commercialName={data.article.commercial_name} locationId={data.article.location_id} showMeta userId={user?.member_id ?? ''} isManager={true} editMode
					onDelete={(_, newIds) => { imageIds = newIds; }}
					onReorder={async (newIds) => { try { const result = await api.reorderProductImages(data.article.commercial_name, data.article.location_id, newIds); imageIds = result.image_ids; } catch { /* ignore */ } }}
				/>
			{/if}
			<div class:mt-2={imageIds.length > 0}>
				<ImageUpload
					commercialName={data.article.commercial_name}
					locationId={data.article.location_id}
					{imageIds}
					userName={user?.name ?? ''}
					userGroup={user?.group_name ?? ''}
					onUpdate={(ids) => imageIds = ids}
				/>
			</div>
		</div>
	{/if}

	<ArticleForm
		mode="edit"
		locations={data.locations}
		categories={data.categories}
		initial={data.article}
		isManager={true}
		userName={user?.name ?? ''}
		userGroup={user?.group_name ?? ''}
		individuallyTrackedEdit={data.article.individually_tracked && !isGroupEdit}
		quantityTrackedEdit={isGroupEdit}
		groupCount={data.groupCount}
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

	{#if isGroupEdit && data.groupItems && data.groupItems.length > 0}
		<div class="mt-6">
			<button onclick={() => showItems = !showItems} class="text-sm text-neutral-500 hover:text-neutral-700">
				{showItems ? 'Dölj enskilda artiklar ▲' : `Visa enskilda artiklar (${data.groupItems.length} st) ▼`}
			</button>
			{#if showItems}
				<div class="mt-2 border rounded divide-y divide-neutral-200 text-sm">
					{#each data.groupItems as item}
						<div class="px-3 py-2 flex flex-wrap items-center gap-x-4 gap-y-1">
							<span class="font-medium text-neutral-700 min-w-24">{item.common_name}</span>
							<span class="text-xs px-2 py-0.5 rounded {statusBadgeClass(item.status)}">{msg(`article_status_${item.status}`) ?? item.status}</span>
							{#if item.purchase_date}
								<span class="text-xs text-neutral-500">Inköpt: {item.purchase_date}</span>
							{/if}
							{#if item.purchase_price}
								<span class="text-xs text-neutral-500">{item.purchase_price} kr</span>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
