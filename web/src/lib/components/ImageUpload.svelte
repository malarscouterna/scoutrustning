<script lang="ts">
	import { createApiClient } from '$lib/api/client';

	interface Props {
		commercialName: string;
		locationId: string;
		imageIds: string[];
		onUpdate: (newIds: string[]) => void;
	}

	let { commercialName, locationId, imageIds, onUpdate }: Props = $props();
	const api = createApiClient();

	let uploading = $state(false);
	let error = $state('');
	let fileInput = $state<HTMLInputElement | null>(null);

	async function handleUpload(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		uploading = true;
		error = '';
		try {
			const result = await api.uploadProductImage(file, commercialName, locationId);
			onUpdate(result.image_ids);
		} catch (e: any) {
			error = e.message ?? 'Uppladdning misslyckades';
		} finally {
			uploading = false;
			if (fileInput) fileInput.value = '';
		}
	}

	async function handleDelete(imageId: string) {
		if (!confirm('Ta bort bilden?')) return;
		error = '';
		try {
			await api.deleteProductImage(imageId, commercialName, locationId);
			onUpdate(imageIds.filter(id => id !== imageId));
		} catch (e: any) {
			error = e.message ?? 'Borttagning misslyckades';
		}
	}
</script>

<div class="space-y-2">
	<span class="text-sm text-neutral-600 block">Produktbilder</span>

	{#if imageIds.length > 0}
		<div class="flex gap-2 overflow-x-auto pb-1">
			{#each imageIds as imgId}
				<div class="shrink-0 relative group w-32">
					<img
						src="/api/v0/images/{imgId}_thumb.webp"
						alt={commercialName}
						class="w-full rounded aspect-[4/3] object-cover"
						loading="lazy"
					/>
					<button
						onclick={() => handleDelete(imgId)}
						class="absolute top-1 right-1 bg-red-600 text-white rounded-full w-5 h-5 text-xs leading-none opacity-0 group-hover:opacity-100 transition-opacity"
						aria-label="Ta bort bild"
					>&times;</button>
				</div>
			{/each}
		</div>
	{/if}

	<label class="inline-flex items-center gap-2 text-sm text-blue-700 border border-blue-200 bg-blue-50 rounded px-3 py-1.5 cursor-pointer hover:bg-blue-100">
		{uploading ? 'Laddar upp...' : '+ Lägg till bild'}
		<input
			bind:this={fileInput}
			type="file"
			accept="image/jpeg,image/png,image/webp,image/heic,image/heif"
			onchange={handleUpload}
			disabled={uploading}
			class="hidden"
		/>
	</label>

	{#if error}
		<p class="text-xs text-red-600">{error}</p>
	{/if}
</div>
