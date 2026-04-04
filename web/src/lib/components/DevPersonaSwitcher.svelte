<script lang="ts">
	import type { User } from '$lib/user';

	let {
		personas,
		currentPersona
	}: { personas: Record<string, User>; currentPersona: string } = $props();

	let open = $state(false);

	const roleLabels: Record<string, string> = {
		leader: 'Ledare',
		project_leader: 'Projektledare',
		equipment_manager: 'Materialare'
	};

	async function switchPersona(key: string) {
		await fetch('/dev/persona', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ persona: key })
		});
		window.location.reload();
	}

	let current = $derived(personas[currentPersona]);
</script>

<div class="fixed bottom-4 right-4 z-50">
	{#if open}
		<div class="bg-white border border-neutral-300 rounded-lg shadow-lg w-72 mb-2">
			<div class="px-3 py-2 border-b bg-neutral-50 rounded-t-lg flex items-center justify-between">
				<span class="text-xs font-medium text-neutral-600">Dev persona</span>
				<button onclick={() => (open = false)} class="text-neutral-400 hover:text-neutral-600 text-sm">✕</button>
			</div>
			<div class="p-1">
				{#each Object.entries(personas) as [key, persona]}
					<button
						onclick={() => switchPersona(key)}
						class="w-full text-left px-3 py-2 rounded text-sm hover:bg-neutral-50 flex items-center justify-between gap-2"
						class:bg-blue-50={key === currentPersona}
					>
						<div>
							<div class="font-medium text-xs">{persona.name}</div>
							<div class="text-xs text-neutral-500">
								{persona.roles.map((r) => roleLabels[r] ?? r).join(', ')}
								{#if persona.units.length > 0}
									· {persona.units.join(', ')}
								{/if}
							</div>
						</div>
						{#if key === currentPersona}
							<span class="text-blue-600 text-xs">●</span>
						{/if}
					</button>
				{/each}
			</div>
		</div>
	{/if}
	<button
		onclick={() => (open = !open)}
		class="bg-neutral-800 text-white px-3 py-2 rounded-full shadow-lg text-xs flex items-center gap-2 hover:bg-neutral-700"
	>
		<span>👤</span>
		<span>{current?.name ?? currentPersona}</span>
	</button>
</div>
