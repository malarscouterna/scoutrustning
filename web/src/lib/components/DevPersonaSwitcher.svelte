<script lang="ts">
	import type { User } from '$lib/user';

	let {
		personas,
		currentPersona,
		user
	}: { personas: Record<string, User>; currentPersona: string | null; user: User | null } = $props();

	let open = $state(false);

	const roleLabels: Record<string, string> = {
		leader: 'Ledare',
		project_leader: 'Projektledare',
		equipment_manager: 'Materialare'
	};

	async function switchPersona(key: string | null) {
		await fetch('/dev/persona', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ persona: key })
		});
		if (key === null) {
			// Go to login page which auto-redirects to Keycloak
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
</script>

<div class="fixed bottom-18 sm:bottom-4 right-4 z-50">
	{#if open}
		<div class="bg-white border border-neutral-300 rounded-lg shadow-lg w-72 mb-2">
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
								{user.name} · {user.roles.map((r) => roleLabels[r] ?? r).join(', ')}
								{#if user.units.length > 0}
									· {user.units.join(', ')}
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
