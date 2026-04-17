<script lang="ts">
	import type { Article } from '$lib/api/client';
	import { statusLabels, statusColors } from '$lib/labels';

	interface Props {
		article: Article;
	}

	let { article }: Props = $props();
</script>

<a href="/articles/{article.id}" class="block border rounded px-4 py-3 hover:bg-neutral-50">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="min-w-0">
			<span class="font-medium text-sm truncate block">{article.common_name}</span>
			<span class="text-xs text-neutral-500 truncate block">({article.commercial_name})</span>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<span class="text-xs px-2 py-0.5 rounded {statusColors[article.status] ?? 'bg-neutral-100'}">{statusLabels[article.status] ?? article.status}</span>
			<span class="text-xs text-neutral-400">{article.location_name}</span>
		</div>
	</div>
</a>
