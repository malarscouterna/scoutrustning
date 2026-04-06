<script lang="ts">
	import '../app.css';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import DevPersonaSwitcher from '$lib/components/DevPersonaSwitcher.svelte';

	if (browser) {
		import('@scouterna/ui-webc/loader');
	}

	let { children, data } = $props();
</script>

{#if data.demo}
	<a href="/guide" class="block bg-adventurerorange-100 border-b border-adventurerorange-300 text-adventurerorange-900 text-center text-sm py-2 px-4 font-medium hover:bg-adventurerorange-200">
		🏕️ Demo — detta är en testmiljö. Bokningar och data kan återställas när som helst.
	</a>
{/if}

{#if data.user}
	<nav class="border-b bg-white sticky top-0 z-10">
		<div class="max-w-4xl mx-auto px-4 py-2 flex items-center gap-4">
			<a href="/" class="font-bold text-blue-800">ms-utrustning</a>
			<a href="/browse" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/browse')}>Utrustning</a>
			<a href="/book" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/book')}>Boka</a>
			<a href="/bookings" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/bookings')}>Bokningar</a>
			<a href="/issues" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/issues')}>Ärenden</a>
			<a href="/guide" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/guide')}>Guide</a>
			<a href="/profile" class="ml-auto text-sm font-medium hover:underline">{data.user.name}</a>
		</div>
	</nav>
{/if}

{@render children()}

{#if data.dev}
	<DevPersonaSwitcher personas={data.dev.personas} currentPersona={data.dev.currentPersona} user={data.user} />
{/if}
