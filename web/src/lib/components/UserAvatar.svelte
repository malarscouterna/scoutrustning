<script lang="ts">
	interface Props {
		name: string;
		picture?: string | null;
		size?: number;
	}

	let { name, picture = null, size = 32 }: Props = $props();

	const initials = $derived(
		name
			.split(/\s+/)
			.filter(Boolean)
			.slice(0, 2)
			.map((part) => part[0]?.toUpperCase() ?? '')
			.join('') || '?'
	);
</script>

{#if picture}
	<img
		src={picture}
		alt={name}
		class="rounded-full object-cover shrink-0"
		style="width: {size}px; height: {size}px;"
	/>
{:else}
	<span
		class="rounded-full bg-neutral-200 text-neutral-600 flex items-center justify-center shrink-0 font-medium"
		style="width: {size}px; height: {size}px; font-size: {size * 0.4}px;"
		aria-hidden="true"
	>
		{initials}
	</span>
{/if}
