<script lang="ts">
	import type { User } from '$lib/user';

	let { user }: { user: User } = $props();

	let open = $state(false);

	async function switchGroup(groupId: string) {
		await fetch('/switch-group', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ groupId })
		});
		window.location.reload();
	}
</script>

<div class="relative">
	{#if open}
		<div class="absolute right-0 top-full mt-2 bg-white border border-neutral-300 rounded-lg shadow-lg w-64 z-50">
			<div class="px-3 py-2 border-b bg-neutral-50 rounded-t-lg flex items-center justify-between">
				<span class="text-xs font-medium text-neutral-600">Byt kår</span>
				<button onclick={() => (open = false)} class="text-neutral-400 hover:text-neutral-600 text-sm">✕</button>
			</div>
			<div class="p-1">
				{#each user.available_groups as group}
					<button
						onclick={() => switchGroup(group.id)}
						class="w-full text-left px-3 py-2 rounded text-sm hover:bg-neutral-50 flex items-center justify-between gap-2"
						class:bg-blue-50={group.id === user.group_id}
					>
						<span class="font-medium text-xs">{group.name}</span>
						{#if group.id === user.group_id}
							<span class="text-blue-600 text-xs">●</span>
						{/if}
					</button>
				{/each}
			</div>
		</div>
	{/if}
	<button
		onclick={() => (open = !open)}
		class="text-white bg-neutral-700 hover:bg-neutral-600 px-3 py-2 rounded-full shadow-lg text-xs flex items-center gap-1"
	>
		<span class="material-symbols-outlined text-sm" style="font-size: 14px;">group</span>
		<span>{user.group_name}</span>
	</button>
</div>
