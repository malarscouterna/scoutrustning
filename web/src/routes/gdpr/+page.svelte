<script lang="ts">
	import type { PageData } from './$types';
	import MarkdownArticle from '$lib/components/MarkdownArticle.svelte';

	let { data }: { data: PageData } = $props();
</script>

<div class="max-w-3xl mx-auto p-4 pb-16">
	<MarkdownArticle html={data.html} />
</div>
