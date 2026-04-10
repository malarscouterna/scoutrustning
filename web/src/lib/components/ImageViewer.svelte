<script lang="ts">
	interface Props {
		src: string;
		thumbSrc: string;
		alt: string;
		downloadId?: string;
		class?: string;
	}

	let { src, thumbSrc, alt, downloadId, class: className = '' }: Props = $props();
	let dialog = $state<HTMLDialogElement | null>(null);

	function open() {
		dialog?.showModal();
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === dialog) dialog?.close();
	}
</script>

<button onclick={open} class="cursor-zoom-in {className}">
	<img {alt} src={thumbSrc} class="w-full rounded aspect-[4/3] object-cover" loading="lazy" />
</button>

<dialog
	bind:this={dialog}
	onclick={handleBackdropClick}
	class="fixed inset-0 m-0 h-dvh w-dvw max-h-dvh max-w-dvw bg-black/90 backdrop:bg-transparent p-0 open:flex items-center justify-center"
>
	<div class="relative w-full h-full flex flex-col items-center justify-center p-4">
		<img {alt} {src} class="max-w-full max-h-[calc(100dvh-5rem)] object-contain" />
		<div class="flex gap-4 mt-3">
			{#if downloadId}
				<a
					href="/api/v0/images/{downloadId}.webp?format=jpeg"
					download
					class="text-sm text-white/80 hover:text-white underline"
				>Ladda ner</a>
			{/if}
			<button onclick={() => dialog?.close()} class="text-sm text-white/80 hover:text-white underline">Stäng</button>
		</div>
		<button
			onclick={() => dialog?.close()}
			class="absolute top-2 right-3 text-white/70 hover:text-white text-2xl leading-none"
			aria-label="Stäng"
		>&times;</button>
	</div>
</dialog>
