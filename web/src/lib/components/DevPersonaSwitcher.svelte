<script lang="ts">
	import type { User } from '$lib/user';

	const accessLabels: Record<string, string> = {
		view: 'Visa',
		book: 'Boka',
		trusted: 'Betrodd',
		manager: 'Ansvarig'
	};

	let {
		personas,
		currentPersona,
		user
	}: { personas: Record<string, any>; currentPersona: string | null; user: User | null } = $props();

	let open = $state(false);

	async function switchPersona(key: string | null) {
		await fetch('/dev/persona', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ persona: key })
		});
		if (key === null) {
			window.location.href = '/login';
		} else {
			window.location.reload();
		}
	}

	let isScoutID = $derived(currentPersona === null);
	let label = $derived(
		isScoutID
			? (user?.name ?? 'ScoutID')
			: (personas[currentPersona!]?.name ?? currentPersona)
	);

	function personaTeamSummary(persona: any): string {
		if (!persona.groups) return '';
		const names: string[] = [];
		for (const teamNames of Object.values(persona.groups) as string[][]) {
			names.push(...teamNames);
		}
		return names.join(', ') || 'Inga team';
	}
</script>

<div class="fixed top-2 right-4 sm:bottom-4 sm:top-auto z-50 flex flex-col sm:flex-col-reverse sm:items-end">
	{#if open}
		<div class="bg-white border border-neutral-300 rounded-lg shadow-lg w-72 mt-2 sm:mt-0 sm:mb-2 max-h-[70vh] overflow-y-auto">
			<div class="px-3 py-2 border-b bg-neutral-50 rounded-t-lg flex items-center justify-between">
				<span class="text-xs font-medium text-neutral-600">Dev persona</span>
				<button onclick={() => (open = false)} class="text-neutral-400 hover:text-neutral-600 text-sm">✕</button>
			</div>
			<div class="p-1">
				<button
					onclick={() => switchPersona(null)}
					class="w-full text-left px-3 py-2 rounded text-sm hover:bg-neutral-50 flex items-center justify-between gap-2"
					class:bg-green-50={isScoutID}
				>
					<div>
						<div class="font-medium text-xs">🔑 ScoutID-inloggning</div>
						{#if isScoutID && user}
							<div class="text-xs text-green-700">
								{user.name} · {accessLabels[user.max_access] ?? user.max_access}
								{#if user.teams.length > 0}
									· {user.teams.map(t => t.team_name).join(', ')}
								{/if}
							</div>
						{:else}
							<div class="text-xs text-neutral-500">Riktig OIDC-inloggning</div>
						{/if}
					</div>
					{#if isScoutID}
						<span class="text-green-600 text-xs">●</span>
					{/if}
				</button>
				<div class="border-t my-1"></div>
				{#each Object.entries(personas) as [key, persona]}
					<button
						onclick={() => switchPersona(key)}
						class="w-full text-left px-3 py-2 rounded text-sm hover:bg-neutral-50 flex items-center justify-between gap-2"
						class:bg-blue-50={key === currentPersona}
					>
						<div>
							<div class="font-medium text-xs">{persona.name}</div>
							<div class="text-xs text-neutral-500">
								{personaTeamSummary(persona)}
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
		class="text-white px-3 py-2 rounded-full shadow-lg text-xs flex items-center gap-2"
		class:bg-green-700={isScoutID}
		class:hover:bg-green-600={isScoutID}
		class:bg-neutral-800={!isScoutID}
		class:hover:bg-neutral-700={!isScoutID}
	>
		<span>{isScoutID ? '🔑' : '👤'}</span>
		<span>{label}</span>
	</button>
</div>
