<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import DevPersonaSwitcher from '$lib/components/DevPersonaSwitcher.svelte';
	import FloatingCart from '$lib/components/FloatingCart.svelte';

	let { children, data } = $props();

	interface Section { label: string; href: string }

	let section = $derived.by((): Section | null => {
		const path = $page.url.pathname;
		if (path.startsWith('/bookings') || path.startsWith('/book')) return { label: 'Bokningar', href: '/bookings' };
		if (path.startsWith('/issues')) return { label: 'Ärenden', href: '/issues' };
		if (path.startsWith('/browse') || path.startsWith('/articles')) return { label: 'Utrustning', href: '/browse' };
		return null;
	});

	// True when we're on the section's own index page (the breadcrumb leaf, not a sub-page)
	let onSectionRoot = $derived(section !== null && $page.url.pathname === section.href);
</script>

{#if data.demo}
	<a href="/guide" class="block bg-adventurerorange-100 border-b border-adventurerorange-300 text-adventurerorange-900 text-center text-sm py-2 px-4 font-medium hover:bg-adventurerorange-200">
		🏕️ Demo — detta är en testmiljö. Bokningar och data kan återställas när som helst.
	</a>
{/if}

{#if !data.user && data.oidcName}
	<div class="flex flex-col items-center justify-center min-h-screen px-4 bg-white text-neutral-900">
		<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Utrustningsgruppen" class="w-48 mb-6" />
		<h1 class="text-xl font-bold mb-2">Hej {data.oidcName}!</h1>
		{#if data.demo}
			<p class="text-sm text-neutral-600 mb-4 max-w-sm text-center">Din scoutkår är inte konfigurerad i den här demomiljön. Använd persona-väljaren nedan för att testa systemet.</p>
		{:else}
			<p class="text-sm text-neutral-600 mb-4 max-w-sm text-center">Din scoutkår är inte konfigurerad för det här systemet. Kontakta din utrustningsansvarige om du tror att det är fel.</p>
		{/if}
	</div>
{:else if data.user}
	<nav class="sticky top-0 z-10 bg-white border-b">
		<div class="max-w-4xl mx-auto px-4 py-2 flex items-center justify-between">
			<a href="/">
				<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Hem" class="w-10 h-10 object-contain" />
			</a>
			<div class="flex items-center gap-1 text-sm text-neutral-500" aria-label="Brödsmulor">
				<a href="/" class="hover:text-neutral-900 {$page.url.pathname === '/' ? 'font-medium text-neutral-900' : ''}">Hem</a>
				{#if section}
					<span class="select-none">/</span>
					{#if onSectionRoot}
						<span class="font-medium text-neutral-900">{section.label}</span>
					{:else}
						<a href={section.href} class="hover:text-neutral-900">{section.label}</a>
					{/if}
				{/if}
			</div>
		</div>
	</nav>
{/if}

<div>
	{#if !data.oidcName || data.user}
		{@render children()}
	{/if}
</div>

{#if data.user}
	<FloatingCart />
{/if}

{#if data.dev}
	<DevPersonaSwitcher personas={data.dev.personas} currentPersona={data.dev.currentPersona} user={data.user} />
{/if}
