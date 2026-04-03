<script>
	import '../app.css';
	import { browser } from '$app/environment';

	if (browser) {
		import('@scouterna/ui-webc/loader');
	}

	let { children } = $props();
</script>

{@render children()}
