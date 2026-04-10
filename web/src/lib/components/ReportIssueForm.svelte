<script lang="ts">
	import { createApiClient } from '$lib/api/client';

	interface Props {
		articleId: string;
		articleName: string;
		onReported: (newStatus: string) => void;
		onCancel: () => void;
	}

	let { articleId, articleName, onReported, onCancel }: Props = $props();
	const api = createApiClient();

	let description = $state('');
	let status = $state('reported_usable');
	let error = $state('');
	let submitting = $state(false);

	const options = [
		{ value: 'reported_usable', label: 'Användbar', color: 'bg-orange-600', resultLabel: 'rapporterad — användbar' },
		{ value: 'reported_unusable', label: 'Ej användbar', color: 'bg-red-600', resultLabel: 'rapporterad — ej användbar' },
		{ value: 'lost', label: 'Saknas', color: 'bg-challengerpink-600', resultLabel: 'saknas' },
	];

	let resultLabel = $derived(options.find((o) => o.value === status)?.resultLabel ?? status);

	async function submit() {
		if (!description.trim()) { error = 'Beskriv problemet'; return; }
		error = '';
		submitting = true;
		try {
			await api.updateArticleStatus(articleId, { status, comment: description.trim() });
			onReported(status);
		} catch (e: any) {
			error = e.message;
		} finally {
			submitting = false;
		}
	}
</script>

<div class="border rounded p-3 bg-neutral-50 text-sm space-y-2">
	<p class="font-medium">Rapportera problem — {articleName}</p>
	{#if error}<p class="text-red-600 text-xs">{error}</p>{/if}
	<textarea bind:value={description} placeholder="Beskriv problemet..." rows="2" class="block border rounded px-2 py-1 text-sm w-full"></textarea>
	<div class="flex flex-wrap gap-2 items-center">
		{#each options as opt}
			<button onclick={() => status = opt.value} class="text-xs px-3 py-1 rounded border" class:text-white={status === opt.value} class:bg-orange-600={status === opt.value && opt.value === 'reported_usable'} class:bg-red-600={status === opt.value && opt.value === 'reported_unusable'} class:bg-challengerpink-600={status === opt.value && opt.value === 'lost'}>{opt.label}</button>
		{/each}
	</div>
	<p class="text-xs text-neutral-500">→ status: {resultLabel}</p>
	<div class="flex gap-2">
		<button onclick={submit} disabled={submitting} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">Skicka</button>
		<button onclick={onCancel} class="text-xs text-neutral-600 underline">Avbryt</button>
	</div>
</div>
