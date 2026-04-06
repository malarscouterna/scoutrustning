<script lang="ts">
	let { data } = $props();
</script>

<div class="max-w-4xl mx-auto p-4">
	<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Utrustningsgruppen" class="w-48 mb-2" />
	<h1 class="text-heading-sm font-bold">ms-utrustning</h1>
	<p class="text-sm text-neutral-500 mb-4">Utrustningsbokning för Mälarscouterna</p>
	<nav class="space-y-2">
		<a href="/browse" class="block underline text-blue-700">Bläddra utrustning →</a>
		<a href="/book" class="block underline text-blue-700">Boka utrustning →</a>
		<a href="/bookings" class="block underline text-blue-700">Mina bokningar →</a>
		<a href="/issues" class="block underline text-blue-700">Ärenden →</a>
		<a href="/guide" class="block underline text-blue-700">Användarguide →</a>
		{#if data.user}
			<a href="/profile" class="block underline text-blue-700">{data.user.name} →</a>
		{/if}
	</nav>
</div>
