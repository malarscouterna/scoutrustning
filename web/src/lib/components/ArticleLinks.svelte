<script lang="ts">
	import type { ArticleLink } from '#lib/api/client.ts';
	import * as m from '#lib/paraglide/messages.js';

	let { links = [] }: { links?: ArticleLink[] } = $props();
</script>

{#if links.length > 0}
	<div>
		<span class="font-medium text-neutral-500">{m.lbl_links()}:</span>
		<div class="mt-0.5 space-y-0.5">
			{#each links as link (link.id)}
				<a
					href={link.url}
					target="_blank"
					rel="noopener noreferrer"
					class="block text-sm text-blue-700 hover:underline break-all">{link.label}</a>
			{/each}
		</div>
	</div>
{/if}
