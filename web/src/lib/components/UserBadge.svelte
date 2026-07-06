<script lang="ts">
	import UserAvatar from '$lib/components/UserAvatar.svelte';
	import UserInfoCard from '$lib/components/UserInfoCard.svelte';

	interface Props {
		userId: string;
		name: string;
		picture?: string | null;
		contextBookingId?: string;
		size?: number;
	}

	let { userId, name, picture = null, contextBookingId, size = 24 }: Props = $props();

	let open = $state(false);
</script>

<button
	type="button"
	class="inline-flex items-center gap-2 hover:underline"
	onclick={() => (open = true)}
>
	<UserAvatar {name} {picture} {size} />
	<span class="text-sm">{name}</span>
</button>

<UserInfoCard bind:open {userId} {contextBookingId} />
