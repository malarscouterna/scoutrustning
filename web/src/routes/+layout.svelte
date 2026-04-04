<script>
	import '../app.css';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import DevPersonaSwitcher from '$lib/components/DevPersonaSwitcher.svelte';

	if (browser) {
		import('@scouterna/ui-webc/loader');
	}

	let { children, data } = $props();
</script>

<nav class="border-b bg-white sticky top-0 z-10">
	<div class="max-w-4xl mx-auto px-4 py-2 flex items-center gap-4">
		<a href="/" class="font-bold text-blue-800">ms-utrustning</a>
		<a href="/browse" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/browse')}>Utrustning</a>
		<a href="/book" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/book')}>Boka</a>
		<a href="/bookings" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/bookings')}>Bokningar</a>
		<a href="/issues" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/issues')}>Ärenden</a>
	</div>
</nav>

{@render children()}

{#if data.dev}
	<DevPersonaSwitcher personas={data.dev.personas} currentPersona={data.dev.currentPersona} />
{/if}
