<script lang="ts">
	import ImageUploadDialog from './ImageUploadDialog.svelte';
	import SharedImageBrowser from './SharedImageBrowser.svelte';

	interface Props {
		commercialName: string;
		locationId: string;
		imageIds: string[];
		userName: string;
		userGroup: string;
		onUpdate: (newIds: string[]) => void;
	}

	let { commercialName, locationId, imageIds, userName, userGroup, onUpdate }: Props = $props();

	let showUploadDialog = $state(false);
	let showBrowser = $state(false);
	let error = $state('');

	function handleUploadComplete(result: { image_ids: string[] }) {
		showUploadDialog = false;
		onUpdate(result.image_ids);
	}

	function handleBrowseComplete(result: { image_ids: string[] }) {
		showBrowser = false;
		onUpdate(result.image_ids);
	}
</script>

<div class="space-y-2">
	<div class="flex gap-2">
		<button
			type="button"
			onclick={() => showUploadDialog = true}
			class="inline-flex items-center gap-2 text-sm text-blue-700 border border-blue-200 bg-blue-50 rounded px-3 py-1.5 cursor-pointer hover:bg-blue-100"
		>Ladda upp</button>
		<button
			type="button"
			onclick={() => showBrowser = true}
			class="inline-flex items-center gap-2 text-sm text-neutral-600 border border-neutral-200 bg-neutral-50 rounded px-3 py-1.5 cursor-pointer hover:bg-neutral-100"
		>Bläddra</button>
	</div>

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

{#if showBrowser}
	<SharedImageBrowser
		{commercialName}
		{locationId}
		imageIndex={imageIds.length + 1}
		existingFileIds={imageIds}
		onComplete={handleBrowseComplete}
		onCancel={() => showBrowser = false}
	/>
{/if}
