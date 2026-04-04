<script lang="ts">
	import { page } from '$app/stores';

	const callbackUrl = $derived($page.url.searchParams.get('callbackUrl') || '/');
</script>

<div class="flex flex-col items-center justify-center min-h-screen px-4 bg-white text-neutral-900">
	<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Utrustningsgruppen" class="w-48 mb-6" />
	<h1 class="text-xl font-bold mb-1">ms-utrustning</h1>
	<p class="text-sm text-neutral-500 mb-8">Utrustningsbokning för Mälarscouterna</p>

	<form method="POST" action="/auth/signin/keycloak">
		<input type="hidden" name="callbackUrl" value={callbackUrl} />
		<button
			type="submit"
			class="flex items-center gap-3 bg-white border border-neutral-300 rounded-lg px-5 py-3 shadow-sm hover:shadow-md hover:border-neutral-400 transition-all"
		>
			<img
				src="https://dev.id.scouterna.se/resources/rip1y/login/scoutid/dist/assets/scoutid-BKqgYpbU.png"
				alt="ScoutID"
				class="h-6"
			/>
			<span class="text-sm font-medium">Logga in med ScoutID</span>
		</button>
	</form>
</div>
