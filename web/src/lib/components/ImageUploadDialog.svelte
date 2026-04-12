<script lang="ts">
	import { onMount } from 'svelte';
	import ImageCropDialog from './ImageCropDialog.svelte';
	import { createApiClient } from '$lib/api/client';

	interface Props {
		commercialName: string;
		locationId: string;
		imageIndex: number;
		userName: string;
		userGroup: string;
		onComplete: (result: { image_ids: string[] }) => void;
		onCancel: () => void;
	}

	let { commercialName, locationId, imageIndex, userName, userGroup, onComplete, onCancel }: Props = $props();
	const api = createApiClient();

	let step = $state<'pick' | 'crop' | 'meta'>('pick');
	let imageSrc = $state('');
	let croppedBlob = $state<Blob | null>(null);
	let croppedPreview = $state('');
	let format = $state('landscape');
	let title = $state('');

	$effect(() => {
		title = `${commercialName} ${imageIndex}`;
	});
	let description = $state('');
	let shared = $state(false);
	let attributionMode = $state<'first_name' | 'full_name' | 'custom'>('first_name');
	let customAttribution = $state('');
	let uploading = $state(false);
	let error = $state('');
	let fileInput = $state<HTMLInputElement | null>(null);

	let firstNameOnly = $derived(userName.split(' ')[0] || userName);
	let attributionPreview = $derived(
		attributionMode === 'custom' ? customAttribution
		: attributionMode === 'full_name' ? `${userName}, ${userGroup}`
		: `${firstNameOnly}, ${userGroup}`
	);

	onMount(() => {
		fileInput?.click();
	});

	function handleFileSelect(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) {
			handleClose();
			return;
		}
		imageSrc = URL.createObjectURL(file);
		step = 'crop';
	}

	function handleCropConfirm(blob: Blob, fmt: string) {
		croppedBlob = blob;
		format = fmt;
		croppedPreview = URL.createObjectURL(blob);
		step = 'meta';
	}

	function handleCropCancel() {
		URL.revokeObjectURL(imageSrc);
		imageSrc = '';
		step = 'pick';
		if (fileInput) fileInput.value = '';
	}

	async function handleUpload() {
		if (!croppedBlob) return;
		uploading = true;
		error = '';
		try {
			const result = await api.uploadProductImage(croppedBlob, commercialName, locationId, {
				title,
				description,
				format,
				shared,
				attribution: attributionPreview,
			});
			onComplete(result);
		} catch (e: any) {
			error = e.message ?? 'Uppladdning misslyckades';
		} finally {
			uploading = false;
		}
	}

	function handleClose() {
		if (imageSrc) URL.revokeObjectURL(imageSrc);
		if (croppedPreview) URL.revokeObjectURL(croppedPreview);
		onCancel();
	}
</script>

{#if step === 'crop'}
	<ImageCropDialog
		{imageSrc}
		onConfirm={handleCropConfirm}
		onCancel={handleCropCancel}
	/>
{:else if step === 'meta'}
	<div class="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
		<div class="bg-white rounded-lg shadow-xl max-w-md w-full max-h-[90vh] overflow-y-auto">
			<div class="p-4 space-y-4">
				<h3 class="font-medium text-sm">Ladda upp bild</h3>

				{#if croppedPreview}
					<div class="relative">
						<img src={croppedPreview} alt="Förhandsgranskning" class="w-full rounded max-h-48 object-contain bg-neutral-100" />
						<button type="button" onclick={() => step = 'crop'} class="absolute bottom-2 right-2 bg-white/80 text-xs px-2 py-1 rounded shadow hover:bg-white">Beskär igen</button>
					</div>
				{/if}

				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Titel</span>
					<input type="text" bind:value={title} class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>

				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
					<textarea bind:value={description} rows={2} class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
				</label>

				<div class="space-y-3">
					<fieldset class="space-y-1.5 text-xs text-neutral-600">
						<legend class="text-sm text-neutral-600 mb-1">Fotograf</legend>
						<label class="flex items-center gap-2">
							<input type="radio" bind:group={attributionMode} value="first_name" />
							{firstNameOnly}, {userGroup}
						</label>
						<label class="flex items-center gap-2">
							<input type="radio" bind:group={attributionMode} value="full_name" />
							{userName}, {userGroup}
						</label>
						<label class="flex items-center gap-2">
							<input type="radio" bind:group={attributionMode} value="custom" />
							Egen text
						</label>
						{#if attributionMode === 'custom'}
							<input type="text" bind:value={customAttribution} placeholder="T.ex. namn eller organisation" class="border rounded px-2 py-1 text-xs w-full mt-1" />
						{/if}
					</fieldset>

					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={shared} />
						Dela med andra scoutkårer
					</label>

					{#if shared}
						<p class="ml-6 text-xs text-neutral-500">Du behöver ha tagit bilden själv eller ha tillåtelse att dela den. Alla identifierbara personer måste ha gett sitt godkännande. Bilder delas enbart bakom scoutinloggning.</p>
					{/if}
				</div>

				<p class="text-xs text-neutral-400">Bilder komprimeras för webben. Originalet sparas inte.</p>

				{#if error}
					<p class="text-xs text-red-600">{error}</p>
				{/if}

				<div class="flex gap-2 pt-2">
					<button
						type="button"
						onclick={handleUpload}
						disabled={uploading}
						class="bg-blue-700 text-white px-4 py-2 rounded text-sm disabled:opacity-50"
					>{uploading ? 'Laddar upp...' : 'Ladda upp'}</button>
					<button type="button" onclick={handleClose} class="text-sm text-neutral-500 underline">Avbryt</button>
				</div>
			</div>
		</div>
	</div>
{:else}
	<!-- step === 'pick': auto-triggered on mount -->
	<input
		bind:this={fileInput}
		type="file"
		accept="image/jpeg,image/png,image/webp,image/heic,image/heif"
		onchange={handleFileSelect}
		class="hidden"
	/>
{/if}
