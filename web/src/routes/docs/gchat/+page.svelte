<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<div class="max-w-3xl mx-auto p-4 pb-16">
	<article class="guide">
		{@html data.html}
	</article>
</div>

<style>
	.guide :global(h1) { font-size: 1.5rem; font-weight: 700; margin-bottom: 0.5rem; }
	.guide :global(h2) { font-size: 1.25rem; font-weight: 700; margin-top: 2rem; margin-bottom: 0.5rem; padding-bottom: 0.25rem; border-bottom: 1px solid #e5e5e5; }
	.guide :global(h3) { font-size: 1.1rem; font-weight: 600; margin-top: 1.5rem; margin-bottom: 0.5rem; }
	.guide :global(p) { margin-bottom: 0.75rem; line-height: 1.6; }
	.guide :global(ul), .guide :global(ol) { margin-bottom: 0.75rem; padding-left: 1.5rem; }
	.guide :global(ul) { list-style: disc; }
	.guide :global(ol) { list-style: decimal; }
	.guide :global(li) { margin-bottom: 0.25rem; line-height: 1.6; }
	.guide :global(strong) { font-weight: 600; }
	.guide :global(code) { background: #f5f5f5; padding: 0.1rem 0.3rem; border-radius: 3px; font-size: 0.875em; }
	.guide :global(pre) { background: #f5f5f5; padding: 1rem; border-radius: 6px; overflow-x: auto; margin-bottom: 1rem; }
	.guide :global(pre code) { background: none; padding: 0; }
	.guide :global(hr) { margin: 2rem 0; border-color: #e5e5e5; }
	.guide :global(a) { color: #1d4ed8; text-decoration: underline; }
</style>
