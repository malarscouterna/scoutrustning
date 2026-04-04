<script lang="ts">
	import type { BookingItem } from '$lib/api/client';

	interface Props {
		items: BookingItem[];
		editable?: boolean;
		onRemove?: (itemId: string) => void;
	}

	let { items, editable = false, onRemove }: Props = $props();

	interface ItemGroup {
		commercialName: string;
		requiresApproval: boolean;
		items: BookingItem[];
	}

	let groups = $derived.by(() => {
		const map = new Map<string, ItemGroup>();
		for (const item of items) {
			const existing = map.get(item.commercial_name);
			if (existing) {
				existing.items.push(item);
				if (item.requires_approval) existing.requiresApproval = true;
			} else {
				map.set(item.commercial_name, {
					commercialName: item.commercial_name,
					requiresApproval: item.requires_approval,
					items: [item]
				});
			}
		}
		return [...map.values()];
	});
</script>

{#if items.length === 0}
	<p class="text-neutral-500">Inga artiklar.</p>
{:else}
	<div class="space-y-2">
		{#each groups as group}
			<div class="border rounded">
				<div class="px-4 py-2 font-medium bg-neutral-50 border-b">
					{group.commercialName} × {group.items.length}
					{#if group.requiresApproval}
						<span class="text-xs bg-orange-100 text-orange-700 px-1.5 py-0.5 rounded ml-1">Kräver godkännande</span>
					{/if}
				</div>
				<table class="w-full text-sm">
					<tbody>
						{#each group.items as item}
							<tr class="border-t first:border-t-0">
								<td class="px-4 py-2">{item.common_name}</td>
								<td class="px-4 py-2 text-neutral-600">{item.location_name}</td>
								<td class="px-4 py-2 text-neutral-600">{item.place || ''}</td>
								{#if item.return_status && item.return_status !== 'returned_ok' && item.return_status !== 'pending'}
									<td class="px-4 py-2">
										<span class="text-xs px-1.5 py-0.5 rounded bg-red-100 text-red-700"
										>{{returned_ok: 'OK', delayed: 'Försenad', reported_usable: 'Problem — användbar', reported_unusable: 'Problem — ej användbar', lost: 'Saknas'}[item.return_status] ?? item.return_status}</span>
									</td>
								{:else if !item.pickup_status || item.pickup_status === 'lost'}
									<td class="px-4 py-2"><span class="text-xs text-neutral-400">Ej hämtad</span></td>
								{:else}
									<td></td>
								{/if}
								{#if editable && onRemove}
									<td class="px-4 py-2 text-right">
										<button onclick={() => onRemove(item.id)} class="text-red-600 text-xs hover:underline">Ta bort</button>
									</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/each}
	</div>
{/if}
