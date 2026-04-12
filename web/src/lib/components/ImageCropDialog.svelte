<script lang="ts">
	import Cropper from 'svelte-easy-crop';
	import type { CropArea } from 'svelte-easy-crop';
	import { cropImage } from '$lib/images/crop';

	interface Props {
		imageSrc: string;
		onConfirm: (blob: Blob, format: string) => void;
		onCancel: () => void;
	}

	let { imageSrc, onConfirm, onCancel }: Props = $props();

	type Format = 'landscape' | 'portrait' | 'square';
	const formats: { value: Format; label: string; aspect: number }[] = [
		{ value: 'landscape', label: 'Liggande', aspect: 4 / 3 },
		{ value: 'portrait', label: 'Stående', aspect: 3 / 4 },
		{ value: 'square', label: 'Kvadrat', aspect: 1 },
	];

	let format = $state<Format>('landscape');
	let crop = $state({ x: 0, y: 0 });
	let zoom = $state(1);
	let pixelCrop = $state<CropArea | null>(null);
	let cropping = $state(false);
	let autoDetected = $state(false);

	// Auto-detect best format from image dimensions
	$effect(() => {
		if (autoDetected) return;
		const img = new Image();
		img.onload = () => {
			const ratio = img.naturalWidth / img.naturalHeight;
			if (ratio > 1.15) format = 'landscape';
			else if (ratio < 0.85) format = 'portrait';
			else format = 'square';
			autoDetected = true;
		};
		img.src = imageSrc;
	});

	let aspect = $derived(formats.find(f => f.value === format)!.aspect);

	function handleCropComplete(e: { pixels: CropArea }) {
		pixelCrop = e.pixels;
	}

	async function handleConfirm() {
		if (!pixelCrop) return;
		cropping = true;
		try {
			const blob = await cropImage(imageSrc, pixelCrop);
			onConfirm(blob, format);
		} finally {
			cropping = false;
		}
	}
</script>

<div class="fixed inset-0 bg-black/70 z-50 flex flex-col">
	<div class="flex items-center justify-between px-4 py-2 bg-neutral-900 text-white text-sm">
		<button
			type="button"
			onclick={onCancel} class="underline">Avbryt</button>
		<div class="flex gap-1">
			{#each formats as f}
				<button
					type="button"
					onclick={() => { format = f.value; crop = { x: 0, y: 0 }; zoom = 1; }}
					class="px-3 py-1 rounded text-xs {format === f.value ? 'bg-white text-black' : 'bg-neutral-700'}"
				>{f.label}</button>
			{/each}
		</div>
		<button
			type="button"
			onclick={handleConfirm}
			disabled={!pixelCrop || cropping}
			class="bg-blue-600 px-3 py-1 rounded disabled:opacity-50"
		>{cropping ? 'Beskär...' : 'Beskär'}</button>
	</div>

	<div class="relative flex-1">
		<Cropper
			image={imageSrc}
			bind:crop
			bind:zoom
			{aspect}
			oncropcomplete={handleCropComplete}
		/>
	</div>

	<div class="px-4 py-2 bg-neutral-900 flex justify-center">
		<label class="flex items-center gap-2 text-white text-xs">
			<input type="range" min={1} max={3} step={0.01} bind:value={zoom} class="w-40" />
			Zoom
		</label>
	</div>
</div>
