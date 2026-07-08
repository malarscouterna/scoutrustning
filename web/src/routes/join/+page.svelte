<script lang="ts">
	import * as m from '$lib/paraglide/messages.js';
	import { createApiClient } from '$lib/api/client';

	let { data } = $props();

	let orgId = $state('');
	let groupName = $state('');
	let roleId = $state<number | null>(null);
	let roleNameFreeText = $state('');
	let teamName = $state('Utrustningsgruppen');
	let contactEmail = $state('');
	let groupSize = $state('');
	let interestedInDomain = $state(false);
	let tosAccepted = $state(false);

	$effect(() => {
		orgId = data.orgs[0]?.id ?? '';
	});

	let selectedOrgRoles = $derived(data.orgs.find((o) => o.id === orgId)?.roles ?? []);
	let selectedRole = $derived(selectedOrgRoles.find((r) => r.id === roleId));
	let roleName = $derived(selectedRole?.name ?? roleNameFreeText);
	let roleKey = $derived(selectedRole?.key ?? '');

	$effect(() => {
		const org = data.orgs.find((o) => o.id === orgId);
		groupName = org?.name ?? '';
		roleId = org?.roles[0]?.id ?? null;
	});

	let submitting = $state(false);
	let error = $state('');
	let sent = $state(false);

	const api = createApiClient();

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (data.demo || !tosAccepted || submitting) return;
		submitting = true;
		error = '';
		try {
			await api.submitJoin({
				org_id: orgId,
				group_name: groupName,
				role_name: roleName,
				role_key: roleKey,
				team_name: teamName,
				contact_email: contactEmail,
				group_size: groupSize,
				interested_in_custom_domain: interestedInDomain
			});
			sent = true;
		} catch {
			error = m.page_join_error();
		} finally {
			submitting = false;
		}
	}
</script>

<div class="min-h-screen bg-white text-neutral-900 flex flex-col items-center px-4 py-12">
	<div class="w-full max-w-md">
		<img src="/PNG Utrustningsgruppen - Logotyp.png" alt="Utrustningsgruppen" class="w-32 mb-6 mx-auto" />

		<a href="/welcome" class="inline-flex items-center gap-1 text-sm text-neutral-500 hover:text-neutral-800 mb-6">
			← {m.page_join_back_to_welcome()}
		</a>

		{#if data.demo}
			<div class="bg-adventurerorange-50 border border-adventurerorange-200 rounded-lg px-4 py-3 mb-6 text-sm text-adventurerorange-900">
				<p class="font-medium mb-1">{m.page_join_demo_heading()}</p>
				<p>{m.page_join_demo_desc()}</p>
			</div>
		{/if}

		{#if sent}
			<div class="text-center space-y-3">
				<h1 class="text-xl font-bold">{m.page_join_success_title()}</h1>
				<p class="text-sm text-neutral-600">{m.page_join_success_body()}</p>
				<a href="/welcome" class="inline-block mt-4 text-sm text-blue-700 underline underline-offset-2">
					{m.page_welcome_title()}
				</a>
			</div>
		{:else}
			<h1 class="text-xl font-bold mb-1 text-center">{m.page_join_title()}</h1>
			<p class="text-sm text-neutral-500 mb-4 text-center">{m.page_join_intro()}</p>

			{#if data.name && data.email}
				<p class="text-xs text-neutral-400 mb-6 text-center">
					{m.page_join_applicant_note({ name: data.name, email: data.email })}
				</p>
			{/if}

			<form onsubmit={submit} class="space-y-5">
			<fieldset disabled={data.demo} class="space-y-5 border-0 p-0 m-0 min-w-0" class:opacity-50={data.demo}>
				{#if data.orgs.length > 1}
					<div>
						<label class="block text-sm font-medium text-neutral-700 mb-1" for="org">{m.page_join_field_org()}</label>
						<select id="org" bind:value={orgId} class="w-full border rounded-lg px-3 py-2 text-sm">
							{#each data.orgs as org (org.id)}
								<option value={org.id}>{org.name ? `${org.name} (${org.id})` : org.id}</option>
							{/each}
						</select>
					</div>
				{/if}

				<div>
					<label class="block text-sm font-medium text-neutral-700 mb-1" for="group-name">{m.page_join_field_group_name()}</label>
					<input id="group-name" type="text" required bind:value={groupName} class="w-full border rounded-lg px-3 py-2 text-sm" />
					<p class="text-xs text-neutral-400 mt-1">{m.page_join_field_group_name_help()}</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-neutral-700 mb-1" for="role-name">{m.page_join_field_role()}</label>
					{#if selectedOrgRoles.length > 0}
						<select id="role-name" required bind:value={roleId} class="w-full border rounded-lg px-3 py-2 text-sm">
							{#each selectedOrgRoles as role (role.id)}
								<option value={role.id}>{role.name} ({role.id})</option>
							{/each}
						</select>
					{:else}
						<input id="role-name" type="text" required bind:value={roleNameFreeText} class="w-full border rounded-lg px-3 py-2 text-sm" />
					{/if}
					<p class="text-xs text-neutral-400 mt-1">{m.page_join_field_role_help()}</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-neutral-700 mb-1" for="team-name">{m.page_join_field_team_name()}</label>
					<input id="team-name" type="text" required bind:value={teamName} class="w-full border rounded-lg px-3 py-2 text-sm" />
					<p class="text-xs text-neutral-400 mt-1">{m.page_join_field_team_name_help()}</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-neutral-700 mb-1" for="contact-email">{m.page_join_field_contact_email()}</label>
					<input id="contact-email" type="email" required bind:value={contactEmail} class="w-full border rounded-lg px-3 py-2 text-sm" />
					<p class="text-xs text-neutral-400 mt-1">{m.page_join_field_contact_email_help()}</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-neutral-700 mb-1" for="group-size">{m.page_join_field_group_size()}</label>
					<input id="group-size" type="text" inputmode="numeric" bind:value={groupSize} class="w-full border rounded-lg px-3 py-2 text-sm" />
				</div>

				<div>
					<label class="flex items-start gap-2 text-sm text-neutral-700">
						<input type="checkbox" bind:checked={interestedInDomain} class="mt-0.5" />
						<span>{m.page_join_field_custom_domain()}</span>
					</label>
					<p class="text-xs text-neutral-400 mt-1">{m.page_join_field_custom_domain_note()}</p>
				</div>

				<div>
					<label class="flex items-start gap-2 text-sm text-neutral-700">
						<input type="checkbox" required bind:checked={tosAccepted} class="mt-0.5" />
						<!-- eslint-disable-next-line svelte/no-at-html-tags -->
						<span>{@html m.page_join_field_tos({
							gdpr_link: `<a href="/gdpr" target="_blank" class="underline underline-offset-2">${m.page_join_field_tos_link()}</a>`
						})}</span>
					</label>
					<p class="text-xs text-neutral-400 mt-1">{m.page_join_pricing_note()}</p>
				</div>

				{#if error}
					<p class="text-sm text-red-600">{error}</p>
				{/if}

				<button
					type="submit"
					disabled={data.demo || submitting || !tosAccepted}
					class="w-full bg-blue-700 text-white rounded-xl px-6 py-3 font-semibold shadow-md hover:shadow-lg disabled:opacity-50 transition-all"
				>
					{m.page_join_submit()}
				</button>
			</fieldset>
			</form>
		{/if}
	</div>
</div>
