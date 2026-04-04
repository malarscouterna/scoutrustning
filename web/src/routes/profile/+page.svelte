<script lang="ts">
	let { data } = $props();
	const user = data.user!;

	const roleConfig: Record<string, { label: string; description: string }> = {
		leader: { label: 'Ledare', description: 'Kan boka utrustning för sina avdelningar' },
		project_leader: { label: 'Projektledare', description: 'Kan boka utrustning utan godkännande' },
		equipment_manager: { label: 'Materialare', description: 'Full tillgång till inventarie, ärenden och godkännanden' }
	};

	const accessGroups = user.roles.map(role => ({
		role,
		label: roleConfig[role]?.label ?? role,
		description: roleConfig[role]?.description ?? '',
		units: user.role_units?.[role] ?? []
	}));
</script>

<div class="max-w-2xl mx-auto px-4 py-8">
	<h1 class="text-xl font-bold mb-1">{user.name}</h1>
	<p class="text-sm text-neutral-500 mb-6">{user.email}</p>

	<h2 class="text-sm font-semibold text-neutral-600 uppercase tracking-wide mb-3">Behörigheter</h2>

	{#if accessGroups.length === 0}
		<p class="text-sm text-neutral-500">Inga roller tilldelade.</p>
	{:else}
		<div class="space-y-4">
			{#each accessGroups as group}
				<div class="border rounded-lg p-4">
					<div class="font-medium">{group.label}</div>
					<div class="text-sm text-neutral-500 mb-2">{group.description}</div>
					{#if group.units.length > 0}
						<div class="flex flex-wrap gap-2">
							{#each group.units as unit}
								<span class="text-xs bg-neutral-100 text-neutral-700 px-2 py-1 rounded">{unit}</span>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}

	<form method="POST" action="/auth/signout" class="mt-8">
		<button type="submit" class="text-sm text-red-600 hover:underline">Logga ut</button>
	</form>
</div>
