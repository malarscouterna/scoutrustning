<script lang="ts">
	let { html }: { html: string } = $props();
</script>

<article class="prose-doc">
	{@html html}
</article>

<style>
	.prose-doc :global(h1) {
		font-size: 1.5rem;
		font-weight: 700;
		margin-bottom: 0.5rem;
	}

	.prose-doc :global(h2) {
		font-size: 1.25rem;
		font-weight: 700;
		margin-top: 2rem;
		margin-bottom: 0.5rem;
		padding-bottom: 0.25rem;
		border-bottom: 1px solid #e5e5e5;
	}

	.prose-doc :global(h3) {
		font-size: 1.1rem;
		font-weight: 600;
		margin-top: 1.5rem;
		margin-bottom: 0.5rem;
	}

	.prose-doc :global(p) {
		margin-bottom: 0.75rem;
		line-height: 1.6;
	}

	.prose-doc :global(ul),
	.prose-doc :global(ol) {
		margin-bottom: 0.75rem;
		padding-left: 1.5rem;
	}

	.prose-doc :global(ul) {
		list-style: disc;
	}

	.prose-doc :global(ol) {
		list-style: decimal;
	}

	.prose-doc :global(li) {
		margin-bottom: 0.25rem;
		line-height: 1.6;
	}

	.prose-doc :global(strong) {
		font-weight: 600;
	}

	.prose-doc :global(hr) {
		margin: 2rem 0;
		border-color: #e5e5e5;
	}

	.prose-doc :global(table) {
		width: 100%;
		border-collapse: collapse;
		margin-bottom: 1rem;
		font-size: 0.875rem;
	}

	.prose-doc :global(th),
	.prose-doc :global(td) {
		border: 1px solid #e5e5e5;
		padding: 0.5rem 0.75rem;
		text-align: left;
	}

	.prose-doc :global(th) {
		background: #f5f5f5;
		font-weight: 600;
	}

	.prose-doc :global(a) {
		color: #1d4ed8;
		text-decoration: underline;
	}

	.prose-doc :global(a:hover) {
		color: #1e40af;
	}
</style>
