<script lang="ts">
	import { createApiClient, type Location, type Category, type GroupSettings } from '$lib/api/client';
	import CrudList from '$lib/components/CrudList.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let user = $derived(data.user!);
	const api = createApiClient();
	let isManager = $derived(user.roles.includes('equipment_manager'));

	const roleConfig: Record<string, { label: string; description: string }> = {
		leader: { label: 'Ledare', description: 'Kan boka utrustning för sina avdelningar' },
		project_leader: { label: 'Projektledare', description: 'Kan boka utrustning utan godkännande' },
		equipment_manager: { label: 'Materialare', description: 'Full tillgång till inventarie, ärenden och godkännanden' }
	};

	let accessGroups = $derived(user.roles.map(role => ({
		role,
		label: roleConfig[role]?.label ?? role,
		description: roleConfig[role]?.description ?? '',
		units: user.role_units?.[role] ?? []
	})));

	type Tab = 'profile' | 'group';
	let tab = $state<Tab>('profile');

	// --- Settings state (local mutable copies, synced from server data) ---
	let locations = $state<Location[]>([]);
	let categories = $state<Category[]>([]);
	let groupSettings = $state<GroupSettings | null>(null);

	$effect(() => {
		locations = data.locations;
		categories = data.categories;
		groupSettings = data.groupSettings;
		if (data.groupSettings) {
			settingsForm.notification_email_from = data.groupSettings.notification_email_from;
			settingsForm.gchat_webhook_url = data.groupSettings.gchat_webhook_url;
		}
	});

	// CSV import
	let importFile = $state<File | null>(null);
	let importResult = $state<any>(null);
	let importLoading = $state(false);
	let importError = $state('');

	// Group notification settings
	let settingsForm = $state({
		notification_email_from: '',
		gchat_webhook_url: '',
		smtp_key: ''
	});

	let settingsMessage = $state('');
	let settingsError = $state('');

	function flash(setter: (v: string) => void, msg: string) {
		setter(msg);
		setTimeout(() => setter(''), 4000);
	}

	// --- CSV Import ---
	function handleFileSelect(e: Event) {
		const input = e.target as HTMLInputElement;
		importFile = input.files?.[0] ?? null;
		importResult = null;
		importError = '';
	}

	async function runImport() {
		if (!importFile) return;
		importLoading = true;
		importError = '';
		importResult = null;
		try {
			importResult = await api.importArticles(importFile);
		} catch (e: any) {
			importError = e.message;
		}
		importLoading = false;
	}

	// --- Group notification settings ---
	async function saveSettings() {
		settingsError = '';
		try {
			const payload: Record<string, any> = {
				notification_email_from: settingsForm.notification_email_from,
				gchat_webhook_url: settingsForm.gchat_webhook_url,
				default_approval_level: 'none'
			};
			if (settingsForm.smtp_key) {
				payload.smtp_key = settingsForm.smtp_key;
			}
			groupSettings = await api.updateGroupSettings(payload);
			settingsForm.smtp_key = '';
			flash(v => settingsMessage = v, 'Inställningar sparade');
		} catch (e: any) {
			settingsError = e.message;
		}
	}
</script>

<div class="max-w-2xl mx-auto px-4 py-8">
	<h1 class="text-xl font-bold mb-1">{user.name}</h1>
	<p class="text-sm text-neutral-500 mb-4">{user.email}</p>

	<!-- Tabs -->
	<div class="flex gap-2 mb-6 border-b">
		<button
			onclick={() => tab = 'profile'}
			class="px-3 py-2 text-sm -mb-px {tab === 'profile' ? 'border-b-2 border-blue-700 font-medium text-blue-700' : 'text-neutral-500'}"
		>Profil</button>
		{#if isManager}
			<button
				onclick={() => tab = 'group'}
				class="px-3 py-2 text-sm -mb-px {tab === 'group' ? 'border-b-2 border-blue-700 font-medium text-blue-700' : 'text-neutral-500'}"
			>Gruppinställningar</button>
		{/if}
	</div>

	<!-- Profile tab -->
	{#if tab === 'profile'}
		<h2 class="text-sm font-semibold text-neutral-600 uppercase tracking-wide mb-3">Behörigheter</h2>

		{#if accessGroups.length === 0}
			<p class="text-sm text-neutral-500">Inga roller tilldelade.</p>
		{:else}
			<div class="space-y-4">
				{#each accessGroups as group}
					<div class="border rounded-lg p-4">
						<div class="font-medium">{group.label}</div>
						<div class="text-sm text-neutral-500 mb-2">{group.description}</div>
						{#if group.units.length > 0}
							<div class="flex flex-wrap gap-2">
								{#each group.units as unit}
									<span class="text-xs bg-neutral-100 text-neutral-700 px-2 py-1 rounded">{unit}</span>
								{/each}
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<h2 class="text-sm font-semibold text-neutral-600 uppercase tracking-wide mt-8 mb-3">Mina inställningar</h2>
		<p class="text-sm text-neutral-500">Personliga inställningar kommer i en framtida version.</p>

		<form method="POST" action="/auth/signout" class="mt-8">
			<button type="submit" class="text-sm text-red-600 hover:underline">Logga ut</button>
		</form>

	<!-- Group settings tab (manager only) -->
	{:else if tab === 'group'}

		<!-- Locations -->
		<section class="mb-8">
			<h3 class="font-medium mb-2">Platser</h3>
			<CrudList
				bind:items={locations}
				label="Plats"
				placeholder="Ny plats..."
				onCreate={(name) => api.createLocation({ name, sort_order: locations.length + 1 })}
				onUpdate={(id, name) => api.updateLocation(id, { name })}
				onDelete={(id) => api.deleteLocation(id)}
			/>
		</section>

		<!-- Categories -->
		<section class="mb-8">
			<h3 class="font-medium mb-2">Kategorier</h3>
			<CrudList
				bind:items={categories}
				label="Kategori"
				placeholder="Ny kategori..."
				onCreate={(name) => api.createCategory({ name, sort_order: categories.length + 1 })}
				onUpdate={(id, name) => api.updateCategory(id, { name })}
				onDelete={(id) => api.deleteCategory(id)}
			/>
		</section>

		<!-- CSV Import -->
		<section class="mb-8">
			<h3 class="font-medium mb-2">Importera artiklar (CSV)</h3>
			<div class="flex flex-wrap items-center gap-2 mb-2">
				<input type="file" accept=".csv" onchange={handleFileSelect} class="text-sm" />
				<button onclick={runImport} disabled={!importFile || importLoading} class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">
					{importLoading ? 'Importerar...' : 'Importera'}
				</button>
			</div>
			{#if importError}
				<div class="bg-red-50 border border-red-200 rounded p-2 text-red-800 text-sm">{importError}</div>
			{/if}
			{#if importResult}
				<div class="bg-green-50 border border-green-200 rounded p-3 text-sm space-y-1">
					<p class="text-green-800 font-medium">{importResult.imported} artiklar importerade</p>
					{#if importResult.skipped > 0}
						<p class="text-orange-700">{importResult.skipped} rader hoppades över</p>
					{/if}
					{#if importResult.errors?.length > 0}
						<details class="text-red-700">
							<summary class="cursor-pointer">{importResult.errors.length} fel</summary>
							<ul class="mt-1 space-y-0.5 text-xs">
								{#each importResult.errors as err}
									<li>{err}</li>
								{/each}
							</ul>
						</details>
					{/if}
				</div>
			{/if}
		</section>

		<!-- Notifications -->
		<section class="mb-8">
			<h3 class="font-medium mb-2">Aviseringar</h3>
			{#if settingsMessage}
				<div class="bg-green-50 border border-green-200 rounded p-2 mb-2 text-green-800 text-sm">{settingsMessage}</div>
			{/if}
			{#if settingsError}
				<div class="bg-red-50 border border-red-200 rounded p-2 mb-2 text-red-800 text-sm">{settingsError}</div>
			{/if}
			<div class="space-y-3">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">E-post avsändaradress</span>
					<input type="email" bind:value={settingsForm.notification_email_from} placeholder="utrustning@example.com" class="border rounded px-2 py-1 text-sm w-full" />
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">
						SMTP-nyckel
						{#if groupSettings?.smtp_key_set}
							<span class="text-neutral-400 ml-1">({groupSettings.smtp_key_masked})</span>
						{/if}
					</span>
					<input type="password" bind:value={settingsForm.smtp_key} placeholder={groupSettings?.smtp_key_set ? 'Lämna tomt för att behålla' : 'Ange SMTP API-nyckel'} class="border rounded px-2 py-1 text-sm w-full" />
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Google Chat webhook-URL</span>
					<input type="url" bind:value={settingsForm.gchat_webhook_url} placeholder="https://chat.googleapis.com/v1/spaces/..." class="border rounded px-2 py-1 text-sm w-full" />
				</label>
				<button onclick={saveSettings} class="text-sm bg-blue-700 text-white px-4 py-2 rounded">Spara</button>
			</div>
		</section>
	{/if}
</div>
