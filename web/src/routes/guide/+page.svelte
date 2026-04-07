<script lang="ts">
	import type { PageData } from './$types';
	import { page } from '$app/stores';

	let { data }: { data: PageData } = $props();
</script>

<div class="max-w-3xl mx-auto p-4 pb-16">
	{#if $page.data.dev}
		<div class="bg-adventurerorange-50 border border-adventurerorange-200 rounded-lg p-4 mb-6">
			<h2 class="font-bold text-adventurerorange-900 mb-2">Demo-läge</h2>
			<p class="text-sm text-adventurerorange-800 mb-3">
				Detta är en demomiljö. Använd personaväljaren (övre högra hörnet på mobil, nedre högra på dator) för att byta mellan olika roller och se systemet från olika perspektiv. Data kan återställas när som helst.
			</p>
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

	<article class="guide">
		{@html data.html}
	</article>
</div>

<style>
	.guide :global(h1) {
		font-size: 1.5rem;
		font-weight: 700;
		margin-bottom: 0.5rem;
	}

	.guide :global(h2) {
		font-size: 1.25rem;
		font-weight: 700;
		margin-top: 2rem;
		margin-bottom: 0.5rem;
		padding-bottom: 0.25rem;
		border-bottom: 1px solid #e5e5e5;
	}

	.guide :global(h3) {
		font-size: 1.1rem;
		font-weight: 600;
		margin-top: 1.5rem;
		margin-bottom: 0.5rem;
	}

	.guide :global(p) {
		margin-bottom: 0.75rem;
		line-height: 1.6;
	}

	.guide :global(ul),
	.guide :global(ol) {
		margin-bottom: 0.75rem;
		padding-left: 1.5rem;
	}

	.guide :global(ul) {
		list-style: disc;
	}

	.guide :global(ol) {
		list-style: decimal;
	}

	.guide :global(li) {
		margin-bottom: 0.25rem;
		line-height: 1.6;
	}

	.guide :global(strong) {
		font-weight: 600;
	}

	.guide :global(hr) {
		margin: 2rem 0;
		border-color: #e5e5e5;
	}

	.guide :global(table) {
		width: 100%;
		border-collapse: collapse;
		margin-bottom: 1rem;
		font-size: 0.875rem;
	}

	.guide :global(th),
	.guide :global(td) {
		border: 1px solid #e5e5e5;
		padding: 0.5rem 0.75rem;
		text-align: left;
	}

	.guide :global(th) {
		background: #f5f5f5;
		font-weight: 600;
	}

	.guide :global(a) {
		color: #1d4ed8;
		text-decoration: underline;
	}

	.guide :global(a:hover) {
		color: #1e40af;
	}
</style>
