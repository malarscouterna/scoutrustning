<script lang="ts">
	import '../app.css';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import DevPersonaSwitcher from '$lib/components/DevPersonaSwitcher.svelte';

	if (browser) {
		import('@scouterna/ui-webc/loader');
	}

	let { children, data } = $props();

	const iconLogo = `<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Hem" style="width:48px;height:48px;object-fit:contain" />`;
	const iconBrowse = `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>`;
	const iconBook = `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v6m3-3H9m12 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>`;
	const iconBookings = `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" /></svg>`;
	const iconIssues = `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" /></svg>`;

	const navItems = [
		{ href: '/', icon: iconLogo, label: '', match: (p: string) => p === '/' },
		{ href: '/browse', icon: iconBrowse, label: 'Utrustn.', match: (p: string) => p.startsWith('/browse') },
		{ href: '/book', icon: iconBook, label: 'Boka', match: (p: string) => p === '/book' },
		{ href: '/bookings', icon: iconBookings, label: 'Bokningar', match: (p: string) => p.startsWith('/bookings') },
		{ href: '/issues', icon: iconIssues, label: 'Ärenden', match: (p: string) => p.startsWith('/issues') },
	];
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
	<!-- Desktop nav -->
	<nav class="hidden sm:block border-b bg-white sticky top-0 z-10">
		<div class="max-w-4xl mx-auto px-4 py-2 flex items-center gap-4">
			<a href="/" class="font-bold text-blue-800">ms-utrustning</a>
			<a href="/browse" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/browse')}>Utrustning</a>
			<a href="/book" class="text-sm hover:underline" class:font-medium={$page.url.pathname === '/book'}>Boka</a>
			<a href="/bookings" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/bookings')}>Bokningar</a>
			<a href="/issues" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/issues')}>Ärenden</a>
			<a href="/guide" class="text-sm hover:underline" class:font-medium={$page.url.pathname.startsWith('/guide')}>Guide</a>
			<a href="/profile" class="ml-auto text-sm font-medium hover:underline">{data.user.name}</a>
		</div>
	</nav>


{/if}

<div class="{data.user ? 'pb-16 sm:pb-0' : ''}">
	{#if !data.oidcName || data.user}
		{@render children()}
	{/if}
</div>

{#if data.user}
	<!-- Mobile bottom bar -->
	<nav class="sm:hidden fixed bottom-0 left-0 right-0 z-10 flex bg-white border-t border-neutral-100">
		{#each navItems as item}
			{@const active = item.match($page.url.pathname)}
			<a
				href={item.href}
				class="flex flex-col items-center justify-center flex-1 py-2 text-neutral-600 no-underline"
				class:text-blue-600={active}
				class:bg-blue-50={active}
			>
				<span class={item.label ? 'w-6 h-6' : ''}>{@html item.icon}</span>
				{#if item.label}<span class="text-xs">{item.label}</span>{/if}
			</a>
		{/each}
	</nav>
{/if}

{#if data.dev}
	<DevPersonaSwitcher personas={data.dev.personas} currentPersona={data.dev.currentPersona} user={data.user} />
{/if}
