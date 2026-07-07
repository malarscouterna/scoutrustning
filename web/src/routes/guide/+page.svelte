<script lang="ts">
	import type { PageData } from './$types';
	import { page } from '$app/stores';
	import MarkdownArticle from '$lib/components/MarkdownArticle.svelte';

	let { data }: { data: PageData } = $props();
</script>

<div class="max-w-3xl mx-auto p-4 pb-16">
	{#if $page.data.dev}
		<div class="bg-adventurerorange-50 border border-adventurerorange-200 rounded-lg p-4 mb-6">
			<h2 class="font-bold text-adventurerorange-900 mb-2">Demo-läge</h2>
			<p class="text-sm text-adventurerorange-800 mb-3">
				Detta är en demomiljö. Använd personaväljaren (övre högra hörnet på mobil, nedre högra på dator) för att byta mellan olika roller och se systemet från olika perspektiv. Data kan återställas när som helst.
			</p>
			{#if data.prodUrl}
				<a
					href={data.prodUrl}
					class="inline-block text-sm font-medium bg-white border border-adventurerorange-300 text-adventurerorange-900 rounded-lg px-4 py-2 hover:bg-adventurerorange-100 transition-colors mb-3"
				>
					Besök den riktiga tjänsten på {data.prodUrl.replace('https://', '')} →
				</a>
			{/if}
			<details class="text-sm">
				<summary class="cursor-pointer font-medium text-adventurerorange-800 mb-2">Tillgängliga personas</summary>
				<table class="w-full text-xs mt-2 border-collapse">
					<thead>
						<tr class="text-left text-adventurerorange-700">
							<th class="py-1 pr-3">Persona</th>
							<th class="py-1 pr-3">Roll</th>
							<th class="py-1">Avdelning/projekt</th>
						</tr>
					</thead>
					<tbody class="text-adventurerorange-900">
						<tr><td class="py-0.5 pr-3">Hanna Yggdrasil</td><td class="pr-3">Ledare</td><td>Yggdrasil</td></tr>
						<tr><td class="py-0.5 pr-3">Fredrik Flaskpost</td><td class="pr-3">Ledare</td><td>Flaskpostorné</td></tr>
						<tr><td class="py-0.5 pr-3">Julia Valborg-Yggdrasil</td><td class="pr-3">Projektledare</td><td>Valborgskommittén + Yggdrasil</td></tr>
						<tr><td class="py-0.5 pr-3">Lovisa Valborg</td><td class="pr-3">Projektledare</td><td>Valborgskommittén</td></tr>
						<tr><td class="py-0.5 pr-3">Gillis Utrustning</td><td class="pr-3">Utrustningsansvarig</td><td>Utrustningsgruppen</td></tr>
						<tr><td class="py-0.5 pr-3">Teo IT-Yggdrasil</td><td class="pr-3">Utrustningsansvarig</td><td>IT-gruppen + Yggdrasil</td></tr>
						<tr><td class="py-0.5 pr-3">Linn Annan-Kår</td><td class="pr-3">Ledare</td><td>Avdelning 1 (annan kår)</td></tr>
					</tbody>
				</table>
				<p class="mt-2 text-adventurerorange-700">
					<strong>Tips:</strong> Prova att byta till Hanna (ledare) och boka utrustning, sen byt till Gillis (utrustningsansvarig) för att godkänna. Byt till Fredrik för att se att han inte ser Yggdrasils bokningar.
				</p>
			</details>
		</div>
	{/if}

	<MarkdownArticle html={data.html} />
</div>
