<script lang="ts">
	import { createApiClient } from '$lib/api/client';
	import ImageUploadDialog from './ImageUploadDialog.svelte';

	interface Props {
		commercialName: string;
		locationId: string;
		imageIds: string[];
		userName: string;
		userGroup: string;
		onUpdate: (newIds: string[]) => void;
	}

	let { commercialName, locationId, imageIds, userName, userGroup, onUpdate }: Props = $props();
	const api = createApiClient();

	let showUploadDialog = $state(false);
	let error = $state('');

	function handleUploadComplete(result: { image_ids: string[] }) {
		showUploadDialog = false;
		onUpdate(result.image_ids);
	}
</script>

<div class="space-y-2">
	<button
		type="button"
		onclick={() => showUploadDialog = true}
		class="inline-flex items-center gap-2 text-sm text-blue-700 border border-blue-200 bg-blue-50 rounded px-3 py-1.5 cursor-pointer hover:bg-blue-100"
	>+ Lägg till bild</button>

	{#if error}
		<p class="text-xs text-red-600">{error}</p>
	{/if}
</div>

{#if showUploadDialog}
	<ImageUploadDialog
		{commercialName}
		{locationId}
		imageIndex={imageIds.length + 1}
		{userName}
		{userGroup}
		onComplete={handleUploadComplete}
		onCancel={() => showUploadDialog = false}
	/>
{/if}
