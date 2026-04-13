<script lang="ts">
	import { createApiClient, type Location, type Category, type GroupSettings } from '$lib/api/client';
	import { browser } from '$app/environment';
	import CrudList from '$lib/components/CrudList.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let user = $derived(data.user!);
	const api = createApiClient();
	let isManager = $derived(user.roles.includes('equipment_manager'));

	const roleConfig: Record<string, { label: string; description: string }> = {
		leader: { label: 'Ledare', description: 'Kan boka utrustning för sina avdelningar' },
		project_leader: { label: 'Projektledare', description: 'Kan boka utrustning utan godkännande' },
		equipment_manager: { label: 'Utrustningsansvarig', description: 'Full tillgång till inventarie, ärenden och godkännanden' }
	};

	let accessGroups = $derived(user.roles.map(role => ({
		role,
		label: roleConfig[role]?.label ?? role,
		description: roleConfig[role]?.description ?? '',
		units: user.role_units?.[role] ?? []
	})));

	let myImages = $state<{ id: string; file_id: string; title: string; description: string; format: string; shared: boolean; created_at: string; own_group_count: number; other_group_count: number }[]>([]);
	let myImagesLoaded = $state(false);
	let expandedImageId = $state<string | null>(null);
	let expandedArticles = $state<{ commercial_name: string; location_name: string; article_id: string }[]>([]);

	let editingMyImage = $state<typeof myImages[0] | null>(null);
	let editMyTitle = $state('');
	let editMyDescription = $state('');
	let editMyShared = $state(false);
	let editMyAttribution = $state('');
	let editMySaving = $state(false);
	let editMyError = $state('');

	function startEditMyImage(img: typeof myImages[0]) {
		editingMyImage = img;
		editMyTitle = img.title;
		editMyDescription = img.description;
		editMyShared = img.shared;
		editMyAttribution = '';
		editMyError = '';
	}

	async function saveEditMyImage() {
		if (!editingMyImage) return;
		editMySaving = true;
		editMyError = '';
		try {
			await api.updateProductImage(editingMyImage.id, {
				title: editMyTitle,
				description: editMyDescription,
				shared: editMyShared,
				attribution: editMyAttribution,
			});
			myImages = myImages.map(i => i.id === editingMyImage!.id ? { ...i, title: editMyTitle, description: editMyDescription, shared: editMyShared } : i);
			editingMyImage = null;
		} catch (e: any) {
			editMyError = e.message ?? 'Kunde inte spara';
		} finally {
			editMySaving = false;
		}
	}

	async function loadMyImages() {
		if (myImagesLoaded) return;
		try {
			myImages = await api.listMyImages();
		} catch { /* ignore */ }
		myImagesLoaded = true;
	}

	async function toggleImageDetail(img: typeof myImages[0]) {
		if (expandedImageId === img.id) {
			expandedImageId = null;
			return;
		}
		expandedImageId = img.id;
		try {
			expandedArticles = await api.listArticlesUsingImage(img.id);
		} catch {
			expandedArticles = [];
		}
	}

	function openFullscreen(img: { file_id: string; format: string }) {
		if (!browser) return;
		const dims: Record<string, { w: number; h: number }> = {
			landscape: { w: 2560, h: 1920 },
			portrait:  { w: 1920, h: 2560 },
			square:    { w: 2048, h: 2048 },
		};
		const d = dims[img.format] ?? { w: 1920, h: 1440 };
		import('photoswipe').then(pswpModule => {
			import('photoswipe/style.css');
			const pswp = new pswpModule.default({
				dataSource: [{ src: `/api/v0/images/${img.file_id}.webp`, width: d.w, height: d.h }],
				index: 0,
				padding: { top: 20, bottom: 40, left: 0, right: 0 },
			});
			pswp.init();
		});
	}

	async function deleteMyImage(img: typeof myImages[0]) {
		const totalRefs = img.own_group_count + img.other_group_count;
		let msg = 'Är du säker på att du vill ta bort bilden?';
		if (totalRefs > 1) {
			const parts: string[] = [];
			if (img.own_group_count > 1) parts.push(`${img.own_group_count - 1} i din kår`);
			if (img.other_group_count > 0) parts.push(`${img.other_group_count} i andra kårer`);
			msg = `Bilden används på ${parts.join(' och ')}. Om du tar bort den försvinner den därifrån också. Fortsätt?`;
		}
		if (!confirm(msg)) return;
		try {
			await api.deleteMyImage(img.id);
			myImages = myImages.filter(i => i.id !== img.id);
			if (expandedImageId === img.id) expandedImageId = null;
		} catch (e: any) {
			alert(e.message ?? 'Borttagning misslyckades');
		}
	}

	$effect(() => {
		if (tab === 'profile') loadMyImages();
	});

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

		<h2 class="text-sm font-semibold text-neutral-600 uppercase tracking-wide mt-8 mb-3">Mina bilder</h2>

		{#if !myImagesLoaded}
			<p class="text-sm text-neutral-400">Laddar...</p>
		{:else if myImages.length === 0}
			<p class="text-sm text-neutral-500">Du har inte laddat upp några bilder ännu.</p>
		{:else}
			<p class="text-xs text-neutral-400 mb-2">{myImages.length} {myImages.length === 1 ? 'bild' : 'bilder'}</p>
			{#if editingMyImage}
				<div class="border rounded-lg p-4 bg-white space-y-3 mb-4">
					<div class="flex items-start gap-4">
						<img src="/api/v0/images/{editingMyImage.file_id}_thumb.webp" alt={editingMyImage.title} class="h-32 rounded object-contain shrink-0" />
						<div class="flex-1 space-y-3 min-w-0">
							<label class="block">
								<span class="text-sm text-neutral-600 block mb-1">Titel</span>
								<input type="text" bind:value={editMyTitle} class="border rounded px-2 py-1.5 text-sm w-full" />
							</label>
							<label class="block">
								<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
								<textarea bind:value={editMyDescription} rows={2} class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
							</label>
							<label class="block">
								<span class="text-sm text-neutral-600 block mb-1">Fotograf</span>
								<input type="text" bind:value={editMyAttribution} class="border rounded px-2 py-1.5 text-sm w-full" />
							</label>
							<label class="flex items-center gap-2 text-sm">
								<input type="checkbox" bind:checked={editMyShared} />
								Dela med andra kårer
							</label>
						</div>
					</div>
					{#if editMyError}
						<p class="text-xs text-red-600">{editMyError}</p>
					{/if}
					<div class="flex gap-2">
						<button type="button" onclick={saveEditMyImage} disabled={editMySaving} class="text-sm bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50">{editMySaving ? 'Sparar...' : 'Spara'}</button>
						<button type="button" onclick={() => editingMyImage = null} class="text-sm text-neutral-500 underline">Avbryt</button>
					</div>
				</div>
			{/if}
			<div class="flex flex-wrap gap-3">
				{#each myImages as img}
					<div class="border rounded overflow-hidden w-[calc(33.333%-0.5rem)]">
						<button type="button" onclick={() => openFullscreen(img)} class="w-full cursor-zoom-in">
							<img
								src="/api/v0/images/{img.file_id}_thumb.webp"
								alt={img.title || 'Bild'}
								class="w-full h-[160px] rounded-t object-contain bg-neutral-50"
								loading="lazy"
							/>
						</button>
						<div class="px-2 py-1.5">
							<p class="text-xs font-medium truncate">{img.title || 'Utan titel'}</p>
							{#if img.shared}
								<span class="text-[10px] bg-blue-100 text-blue-700 px-1 rounded">Delad</span>
							{/if}
							<div class="flex gap-1.5 mt-1">
								<button type="button" onclick={() => toggleImageDetail(img)} class="text-[11px] text-blue-700 border border-blue-200 bg-blue-50 rounded px-1.5 py-0.5 hover:bg-blue-100">
									{expandedImageId === img.id ? 'Dölj' : 'Detaljer'}
								</button>
								<button type="button" onclick={() => startEditMyImage(img)} class="text-[11px] text-blue-700 border border-blue-200 bg-blue-50 rounded px-1.5 py-0.5 hover:bg-blue-100">Redigera</button>
							</div>
						</div>

						{#if expandedImageId === img.id}
							<div class="border-t px-2 py-2 space-y-2 bg-neutral-50">

								{#if img.description}
									<p class="text-xs text-neutral-600">{img.description}</p>
								{/if}

								<p class="text-[10px] text-neutral-400">
									{img.format === 'landscape' ? 'Liggande' : img.format === 'portrait' ? 'Stående' : 'Kvadrat'}
									· {new Date(img.created_at).toLocaleDateString('sv')}
								</p>

								{#if expandedArticles.length > 0}
									<div>
										<span class="text-[10px] font-medium text-neutral-500">Används på:</span>
										{#each expandedArticles as a}
											<a href="/articles/{a.article_id}" class="block text-[10px] text-blue-700 hover:underline">{a.commercial_name} — {a.location_name}</a>
										{/each}
									</div>
								{:else}
									<p class="text-[10px] text-neutral-400">Inte kopplad till någon artikel</p>
								{/if}

								{#if img.other_group_count > 0}
									<p class="text-[10px] text-neutral-400">Används även av {img.other_group_count} {img.other_group_count === 1 ? 'annan kår' : 'andra kårer'}</p>
								{/if}

								<button type="button" onclick={() => deleteMyImage(img)} class="text-[10px] text-red-600 hover:underline">Ta bort bild</button>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

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
