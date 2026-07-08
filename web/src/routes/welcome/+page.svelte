<script lang="ts">
	import { page } from '$app/stores';
	import * as m from '$lib/paraglide/messages.js';

	let { data } = $props();
	const callbackUrl = $derived($page.url.searchParams.get('callbackUrl') || '/');
</script>

<div class="min-h-screen bg-white text-neutral-900 flex flex-col">
	<!-- Hero: logo + login button (or dashboard link if already logged in) always above the fold -->
	<div class="flex flex-col items-center justify-center flex-1 px-4 py-12">
		<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Utrustningsgruppen" class="w-40 mb-5" />
		<h1 class="text-2xl font-bold mb-1 tracking-tight">{m.page_welcome_title()}</h1>
		<p class="text-sm text-neutral-500 mb-8">{m.page_welcome_subtitle()}</p>

		{#if data.user}
			<a
				href="/"
				class="flex items-center justify-center gap-3 w-full max-w-xs bg-white border-2 border-neutral-300 rounded-xl px-6 py-4 shadow-md hover:shadow-lg hover:border-neutral-400 transition-all text-base font-semibold"
			>
				{m.page_welcome_go_to_dashboard()}
			</a>
		{:else if data.oidcName}
			<button
				type="button"
				disabled
				class="flex items-center justify-center gap-3 w-full max-w-xs bg-neutral-100 border-2 border-neutral-200 rounded-xl px-6 py-4 text-base font-semibold text-neutral-400 cursor-not-allowed"
			>
				{m.page_welcome_go_to_dashboard()}
			</button>
			<div class="mt-4 max-w-sm w-full text-sm text-neutral-600 text-center">
				<p class="font-medium mb-1 text-neutral-800">{m.page_welcome_no_group_heading()}</p>
				<p>{m.page_welcome_no_group_desc({ name: data.oidcName })}</p>
			</div>
		{:else}
			{#if data.demo}
				<div class="bg-adventurerorange-50 border border-adventurerorange-200 rounded-lg px-4 py-3 mb-6 max-w-sm w-full text-sm text-adventurerorange-900">
					<p class="font-medium mb-1">{m.page_welcome_demo_heading()}</p>
					<p>{m.page_welcome_demo_desc()}</p>
				</div>
			{/if}

			<form method="POST" action="/auth/signin/keycloak" class="w-full max-w-xs">
				<input type="hidden" name="callbackUrl" value={callbackUrl} />
				<button
					type="submit"
					class="flex items-center justify-center gap-3 w-full bg-white border-2 border-neutral-300 rounded-xl px-6 py-4 shadow-md hover:shadow-lg hover:border-neutral-400 transition-all text-base font-semibold"
				>
					<img
						src="https://dev.id.scouterna.se/resources/rip1y/login/scoutid/dist/assets/scoutid-BKqgYpbU.png"
						alt="ScoutID"
						class="h-6"
					/>
					{m.page_welcome_btn_login()}
				</button>
			</form>
		{/if}
	</div>

	<!-- Informational section below -->
	<div class="border-t border-neutral-100 bg-neutral-50 px-4 py-10">
		<div class="max-w-2xl mx-auto space-y-8">

			<!-- Dev: both links. Demo: prod link only. Prod: demo link only. -->
			{#if (data.demo || data.dev) && data.prodUrl}
				<a
					href={data.prodUrl}
					class="flex items-center justify-between gap-4 border border-neutral-300 rounded-xl px-4 py-3 text-sm text-neutral-700 hover:border-neutral-400 hover:bg-neutral-100 transition-colors"
				>
					<span>{m.page_welcome_try_prod()}</span>
					<span class="text-neutral-400">→</span>
				</a>
			{/if}
			{#if (!data.demo || data.dev) && data.demoUrl}
				<a
					href={data.demoUrl}
					class="flex items-center justify-between gap-4 border border-neutral-300 rounded-xl px-4 py-3 text-sm text-neutral-700 hover:border-neutral-400 hover:bg-neutral-100 transition-colors"
				>
					<span>{m.page_welcome_try_demo()}</span>
					<span class="text-neutral-400">→</span>
				</a>
			{/if}

			<!-- Group signup CTA - users with any session (mapped or unmapped) go straight to /join;
			     logged-out visitors sign in first, landing on /join right after via callbackUrl. -->
			{#if data.user || data.oidcName}
				<a
					href="/join"
					class="flex items-center justify-between gap-4 border border-neutral-300 rounded-xl px-4 py-3 text-sm text-neutral-700 hover:border-neutral-400 hover:bg-neutral-100 transition-colors"
				>
					<span>{m.page_welcome_join_link()}</span>
					<span class="text-neutral-400">→</span>
				</a>
			{:else}
				<form method="POST" action="/auth/signin/keycloak">
					<input type="hidden" name="callbackUrl" value="/join" />
					<button
						type="submit"
						class="flex items-center justify-between gap-4 w-full border border-neutral-300 rounded-xl px-4 py-3 text-sm text-neutral-700 hover:border-neutral-400 hover:bg-neutral-100 transition-colors"
					>
						<span>{m.page_welcome_join_link()}</span>
						<span class="text-neutral-400">→</span>
					</button>
				</form>
			{/if}

			<!-- Description -->
			<p class="text-sm text-neutral-600 leading-relaxed">
				{m.page_welcome_description()}
			</p>

			<!-- How it works -->
			<div>
				<h2 class="text-sm font-semibold text-neutral-800 mb-3">{m.page_welcome_how_it_works_heading()}</h2>
				<ol class="space-y-2 text-sm text-neutral-600">
					<li class="flex gap-3">
						<span class="flex-shrink-0 w-5 h-5 rounded-full bg-neutral-200 text-neutral-700 text-xs font-bold flex items-center justify-center">1</span>
						{m.page_welcome_step_browse()}
					</li>
					<li class="flex gap-3">
						<span class="flex-shrink-0 w-5 h-5 rounded-full bg-neutral-200 text-neutral-700 text-xs font-bold flex items-center justify-center">2</span>
						{m.page_welcome_step_book()}
					</li>
					<li class="flex gap-3">
						<span class="flex-shrink-0 w-5 h-5 rounded-full bg-neutral-200 text-neutral-700 text-xs font-bold flex items-center justify-center">3</span>
						{m.page_welcome_step_pickup()}
					</li>
				</ol>
			</div>

			<!-- Secondary links -->
			<div class="flex flex-wrap gap-x-6 gap-y-3 text-sm">
				<a href="/guide" class="text-neutral-600 hover:text-neutral-900 underline underline-offset-2">
					{m.page_welcome_guide_link()}
				</a>
				<a href="/gdpr" class="text-neutral-600 hover:text-neutral-900 underline underline-offset-2">
					{m.page_welcome_gdpr_link()}
				</a>
			</div>

			<!-- Open source -->
			<p class="text-xs text-neutral-400">
				{m.page_welcome_opensource()}
				<a
					href="https://github.com/malarscouterna/scoutrustning"
					class="underline underline-offset-2 hover:text-neutral-600"
					target="_blank"
					rel="noopener noreferrer"
				>github.com/malarscouterna/scoutrustning</a>
			</p>
		</div>
	</div>
</div>
