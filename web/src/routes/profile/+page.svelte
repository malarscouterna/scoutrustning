<script lang="ts">
	import { createApiClient, type Location, type Category, type GroupSettings, type Team } from '$lib/api/client';
	import { browser } from '$app/environment';
	import CrudList from '$lib/components/CrudList.svelte';
	import type { PageData } from './$types';
	import { msg } from '$lib/msg';

	let { data }: { data: PageData } = $props();
	let user = $derived(data.user);
	const api = createApiClient();
	let mgr = $derived(user?.max_access === 'manager');

	import type { TeamMembership } from '$lib/user';

	let teamsByAccess = $derived(
		(user?.teams ?? []).reduce((acc: Record<string, TeamMembership[]>, t) => {
			(acc[t.access_level] ??= []).push(t);
			return acc;
		}, {} as Record<string, TeamMembership[]>)
	);

	let myImages = $state<{ id: string; file_id: string; title: string; description: string; format: string; shared: boolean; created_at: string; own_group_count: number; other_group_count: number }[]>([]);
	let myImagesLoaded = $state(false);
	let expandedImageId = $state<string | null>(null);
	let expandedArticles = $state<{ commercial_name: string; location_name: string; article_id: string }[]>([]);

	// Team management state
	let allTeams = $state<Team[]>([]);
	$effect(() => { allTeams = data.teams ?? []; });
	const accessLevels = ['view', 'book', 'trusted', 'manager'] as const;
	let editingTeamId = $state<string | null>(null);
	let editingTeamName = $state('');
	let selectedTeamId = $state<string | null>(null);
	let teamError = $state('');
	let showAddTeam = $state(false);
	let newTeam = $state({ name: '', type: 'troop', access_level: 'book', claim_scope: 'troop', claim_id: '' });

	function teamsByLevel(level: string) {
		return allTeams.filter(t => t.access_level === level).sort((a, b) => a.name.localeCompare(b.name));
	}

	async function changeTeamLevel(teamId: string, newLevel: string) {
		teamError = '';
		try {
			const updated = await api.updateTeam(teamId, { access_level: newLevel });
			allTeams = allTeams.map(t => t.id === teamId ? { ...t, ...updated } : t);
		} catch (e: any) { teamError = e.message; }
	}

	async function renameTeam(teamId: string) {
		if (!editingTeamName.trim()) return;
		teamError = '';
		try {
			const updated = await api.updateTeam(teamId, { name: editingTeamName.trim() });
			allTeams = allTeams.map(t => t.id === teamId ? { ...t, ...updated } : t);
			editingTeamId = null;
		} catch (e: any) { teamError = e.message; }
	}

	async function addTeam() {
		if (!newTeam.name.trim() || !newTeam.claim_id.trim()) return;
		teamError = '';
		try {
			const created = await api.createTeam({
				name: newTeam.name.trim(),
				type: newTeam.type,
				access_level: newTeam.access_level,
				claim_scope: newTeam.claim_scope,
				claim_id: newTeam.claim_id.trim()
			});
			allTeams = [...allTeams, created];
			newTeam = { name: '', type: 'troop', access_level: 'book', claim_scope: 'troop', claim_id: '' };
			showAddTeam = false;
		} catch (e: any) { teamError = e.message; }
	}

	async function deleteTeam(teamId: string, teamName: string) {
		if (!confirm(`Ta bort ${teamName}? Användare med denna koppling får en ny avdelning vid nästa inloggning.`)) return;
		teamError = '';
		try {
			await api.deleteTeam(teamId);
			allTeams = allTeams.filter(t => t.id !== teamId);
		} catch (e: any) { teamError = e.message; }
	}

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
		let confirmMsg = 'Är du säker på att du vill ta bort bilden?';
		if (totalRefs > 1) {
			const parts: string[] = [];
			if (img.own_group_count > 1) parts.push(`${img.own_group_count - 1} i din kår`);
			if (img.other_group_count > 0) parts.push(`${img.other_group_count} i andra kårer`);
			confirmMsg = `Bilden används på ${parts.join(' och ')}. Om du tar bort den försvinner den därifrån också. Fortsätt?`;
		}
		if (!confirm(confirmMsg)) return;
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

	// Permission settings
	const permissionConfig = [
		{ key: 'image_upload_role', label: 'Ladda upp bilder', min: 'view' },
		{ key: 'article_edit_role', label: 'Redigera artiklar', min: 'book' },
		{ key: 'issue_resolve_role', label: 'Hantera ärenden', min: 'book' },
		{ key: 'manager_notes_role', label: 'Se interna anteckningar', min: 'trusted' },
	] as const;
	const defaultAccessConfig = [
		{ key: 'default_access_unknown', label: 'Okända användare' },
		{ key: 'default_access_troop', label: 'Nya avdelningar' },
		{ key: 'default_access_role', label: 'Nya roller' },
	] as const;
	let permForm = $state<Record<string, string>>({});
	let permSaving = $state(false);
	let permMessage = $state('');

	$effect(() => {
		if (data.groupSettings) {
			const gs = data.groupSettings;
			permForm = {
				booking_role: gs.booking_role ?? 'book',
				image_upload_role: gs.image_upload_role ?? 'book',
				article_edit_role: gs.article_edit_role ?? 'manager',
				issue_resolve_role: gs.issue_resolve_role ?? 'manager',
				manager_notes_role: gs.manager_notes_role ?? 'manager',
				default_access_unknown: gs.default_access_unknown ?? 'view',
				default_access_troop: gs.default_access_troop ?? 'book',
				default_access_role: gs.default_access_role ?? 'book',
				default_approval_level: gs.default_approval_level ?? 'none',
			};
		}
	});

	async function savePermissions() {
		permSaving = true;
		permMessage = '';
		try {
			await api.updateGroupSettings(permForm as any);
			permMessage = 'Sparat';
			setTimeout(() => permMessage = '', 3000);
		} catch (e: any) {
			permMessage = 'Fel: ' + e.message;
		}
		permSaving = false;
	}

	function allowedLevels(min: string): string[] {
		const all = ['view', 'book', 'trusted', 'manager'];
		const idx = all.indexOf(min);
		return idx >= 0 ? all.slice(idx) : all;
	}

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

	// --- User language preference ---
	let userLanguage = $state<string>('sv');
	$effect(() => { userLanguage = data.user?.language ?? 'sv'; });
	let languageSaving = $state(false);
	let languageMessage = $state('');

	async function saveLanguage() {
		languageSaving = true;
		languageMessage = '';
		try {
			await api.updateLanguage(userLanguage);
			// Set cookie immediately client-side so the next page load activates the language in one step.
			document.cookie = `paraglide_lang=${userLanguage}; path=/; max-age=${60 * 60 * 24 * 365}; samesite=lax`;
			location.reload();
		} catch (e: any) {
			languageMessage = 'Fel: ' + e.message;
			languageSaving = false;
		}
	}

	// --- Group language preference ---
	let groupLanguage = $state<string>('sv');
	let groupLanguageMessage = $state('');
	$effect(() => {
		if (data.groupSettings) groupLanguage = data.groupSettings.default_language ?? 'sv';
	});

	async function saveGroupLanguage() {
		try {
			groupSettings = await api.updateGroupSettings({ default_language: groupLanguage });
			flash(v => groupLanguageMessage = v, 'Sparat');
		} catch (e: any) {
			groupLanguageMessage = 'Fel: ' + e.message;
		}
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

{#if !user}
	<div class="max-w-5xl mx-auto px-4 py-8">
		<p class="text-neutral-500">Laddar...</p>
	</div>
{:else}
<div class="max-w-5xl mx-auto px-4 py-8">
	<h1 class="text-xl font-bold mb-1">{user.name}</h1>
	<p class="text-sm text-neutral-500 mb-4">{user.email}</p>

	<!-- Tabs -->
	<div class="flex gap-2 mb-6 border-b">
		<button
			onclick={() => tab = 'profile'}
			class="px-3 py-2 text-sm -mb-px {tab === 'profile' ? 'border-b-2 border-blue-700 font-medium text-blue-700' : 'text-neutral-500'}"
		>Profil</button>
		{#if mgr}
			<button
				onclick={() => tab = 'group'}
				class="px-3 py-2 text-sm -mb-px {tab === 'group' ? 'border-b-2 border-blue-700 font-medium text-blue-700' : 'text-neutral-500'}"
			>Gruppinställningar</button>
		{/if}
	</div>

	<!-- Profile tab -->
	{#if tab === 'profile'}
		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-3">Behörigheter</h2>

			{#if user.teams.length === 0}
				<p class="text-sm text-neutral-500">Inga avdelningar eller roller tilldelade.</p>
			{:else}
				<div class="space-y-3">
					{#each Object.entries(teamsByAccess) as [level, teams]}
						<div class="bg-neutral-50 rounded-lg px-4 py-3">
							<div class="font-medium text-sm">{msg(`team_access_${level}`) ?? level}</div>
							<div class="text-xs text-neutral-500 mb-2">{msg(`team_access_${level}_description`) ?? ''}</div>
							<div class="flex flex-wrap gap-2">
								{#each teams as team}
									<span class="text-xs bg-white text-neutral-700 px-2 py-1 rounded shadow-sm">
										{team.team_name}
										<span class="text-neutral-400">{team.team_type === 'troop' ? 'Avd.' : 'Roll'}</span>
									</span>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-3">Mina inställningar</h2>
			<div class="flex items-center gap-3">
				<label class="text-sm text-neutral-700" for="user-language">Språk</label>
				<select id="user-language" bind:value={userLanguage} class="border rounded px-2 py-1 text-sm">
					<option value="sv">Svenska{groupLanguage !== 'sv' ? '' : ' (gruppens val)'}</option>
					<option value="en">English{groupLanguage !== 'en' ? '' : ' (group default)'}</option>
				</select>
				<button onclick={saveLanguage} disabled={languageSaving} class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">
					{languageSaving ? 'Sparar...' : 'Spara'}
				</button>
				{#if languageMessage}<span class="text-sm text-green-600">{languageMessage}</span>{/if}
			</div>
			<p class="text-xs text-neutral-400 mt-2">Gäller gränssnittet. Artikelnamn, beskrivningar och annat innehåll som skapats av användare visas alltid på gruppens språk.</p>
		</section>

		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-3">Mina bilder</h2>

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
		</section>

		<form method="POST" action="/auth/signout" class="mt-4">
			<button type="submit" class="text-sm text-red-600 hover:underline">Logga ut</button>
		</form>

	<!-- Group settings tab (manager only) -->
	{:else if tab === 'group'}

		<!-- Teams -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-1">Avdelningar och roller</h3>
			<p class="text-xs text-neutral-500 mb-3">Avdelningar och roller skapas automatiskt när en användare loggar in med en okänd Scoutnet-koppling. Byt namn och ändra åtkomstnivå här.</p>
			{#if teamError}
				<div class="bg-red-50 border border-red-200 rounded p-2 mb-2 text-red-800 text-sm">{teamError}</div>
			{/if}
			<div class="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-3">
				{#each accessLevels as level}
					{@const teams = teamsByLevel(level)}
					<div class="border rounded-lg">
						<div class="px-3 py-1.5 bg-neutral-50 rounded-t-lg border-b">
							<span class="text-sm font-medium">{msg(`team_access_${level}`) ?? level}</span>
							<span class="text-xs text-neutral-400 ml-1">({teams.length})</span>
						</div>
						<div class="p-1.5 space-y-0.5 min-h-[48px]">
							{#each teams as team (team.id)}
								<button
									onclick={() => selectedTeamId = selectedTeamId === team.id ? null : team.id}
									class="w-full text-left px-2 py-1 rounded text-sm flex items-center gap-1.5 hover:bg-neutral-50"
									class:bg-blue-50={selectedTeamId === team.id}
									class:ring-1={selectedTeamId === team.id}
									class:ring-blue-300={selectedTeamId === team.id}
								>
									<span class="truncate flex-1">{team.name}</span>
									<span class="text-[10px] text-neutral-400 shrink-0">{msg(`team_type_${team.type}`) ?? team.type}</span>
								</button>
							{/each}
							{#if teams.length === 0}
								<p class="text-xs text-neutral-400 italic px-2 py-1">Inga</p>
							{/if}
						</div>
					</div>
				{/each}
			</div>

			<!-- Selected team controls -->
			{#if selectedTeamId}
				{@const team = allTeams.find(t => t.id === selectedTeamId)}
				{#if team}
					<div class="border rounded-lg p-3 mb-3 bg-blue-50/50 space-y-2">
						<div class="flex flex-wrap items-center gap-2">
							{#if editingTeamId === team.id}
								<input
									type="text"
									bind:value={editingTeamName}
									onkeydown={(e) => { if (e.key === 'Enter') renameTeam(team.id); if (e.key === 'Escape') editingTeamId = null; }}
									class="border rounded px-2 py-1 text-sm flex-1 min-w-[150px]"
								/>
								<button onclick={() => renameTeam(team.id)} class="text-sm text-blue-700 underline">Spara</button>
								<button onclick={() => editingTeamId = null} class="text-sm text-neutral-500 underline">Avbryt</button>
							{:else}
								<span class="font-medium text-sm">{team.name}</span>
								<span class="text-xs text-neutral-400">{msg(`team_type_${team.type}`) ?? team.type}</span>
								{#if team.claim_mappings?.length > 0}
									<span class="text-xs text-neutral-400">— {team.claim_mappings[0].claim_scope}:{team.claim_mappings[0].claim_id}</span>
								{/if}
							{/if}
						</div>
						{#if editingTeamId !== team.id}
							<div class="flex flex-wrap items-center gap-2">
								<label class="flex items-center gap-1.5 text-sm">
									<span class="text-neutral-600">Åtkomstnivå:</span>
									<select
										value={team.access_level}
										onchange={(e) => changeTeamLevel(team.id, e.currentTarget.value)}
										class="border rounded px-2 py-1 text-sm"
										aria-label="Ändra åtkomstnivå"
									>
										{#each accessLevels as l}
											<option value={l}>{msg(`team_access_${l}`) ?? l}</option>
										{/each}
									</select>
								</label>
								<button onclick={() => { editingTeamId = team.id; editingTeamName = team.name; }} class="text-sm text-blue-700 underline">Byt namn</button>
								<button onclick={() => deleteTeam(team.id, team.name)} class="text-sm text-red-600 underline">Ta bort</button>
							</div>
						{/if}
					</div>
				{/if}
			{/if}

			{#if showAddTeam}
				<div class="border rounded-lg p-3 space-y-2 mb-2">
					<div class="flex flex-wrap gap-2">
						<label class="flex flex-col gap-0.5 flex-1 min-w-[150px]">
							<span class="text-xs text-neutral-500">Namn</span>
							<input type="text" bind:value={newTeam.name} placeholder="T.ex. Yggdrasil" class="border rounded px-2 py-1 text-sm" />
						</label>
						<label class="flex flex-col gap-0.5">
							<span class="text-xs text-neutral-500">Typ</span>
							<select bind:value={newTeam.type} class="border rounded px-2 py-1 text-sm">
								<option value="troop">Avdelning</option>
								<option value="role">Roll</option>
							</select>
						</label>
						<label class="flex flex-col gap-0.5">
							<span class="text-xs text-neutral-500">Åtkomstnivå</span>
							<select bind:value={newTeam.access_level} class="border rounded px-2 py-1 text-sm">
								{#each accessLevels as l}
									<option value={l}>{msg(`team_access_${l}`) ?? l}</option>
								{/each}
							</select>
						</label>
					</div>
					<div class="flex flex-wrap gap-2">
						<label class="flex flex-col gap-0.5">
							<span class="text-xs text-neutral-500">Scoutnet-id</span>
							<select bind:value={newTeam.claim_scope} class="border rounded px-2 py-1 text-sm">
								<option value="troop">Avdelning</option>
								<option value="group">Kårroll</option>
							</select>
						</label>
						<label class="flex flex-col gap-0.5 flex-1 min-w-[100px]">
							<span class="text-xs text-neutral-500">{newTeam.claim_scope === 'troop' ? 'Avdelnings-ID' : 'Rollnamn'} <span class="text-neutral-400">(från Scoutnet)</span></span>
							<input type="text" bind:value={newTeam.claim_id} required placeholder={newTeam.claim_scope === 'troop' ? 'T.ex. 17443' : 'T.ex. it_manager'} class="border rounded px-2 py-1 text-sm" />
						</label>
					</div>
					<p class="text-xs text-neutral-400">Scoutnet-id hittas i organisationsinställningar i Scoutnet.</p>
					<div class="flex gap-2">
						<button onclick={addTeam} class="text-sm bg-blue-700 text-white px-3 py-1 rounded">Lägg till</button>
						<button onclick={() => showAddTeam = false} class="text-sm text-neutral-500 underline">Avbryt</button>
					</div>
				</div>
			{:else}
				<button onclick={() => showAddTeam = true} class="text-sm text-blue-700 underline">+ Lägg till avdelning eller roll</button>
			{/if}
		</section>

		<!-- Permissions -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-1">Behörigheter</h3>
			<p class="text-xs text-neutral-500 mb-3">Vilken åtkomstnivå krävs för varje funktion. Godkänna bokningar och hantera inställningar kräver alltid Ansvarig.</p>
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 mb-3">
				{#each permissionConfig as perm}
					<label class="flex items-center justify-between gap-2 text-sm">
						<span>{perm.label}</span>
						<select bind:value={permForm[perm.key]} class="border rounded px-2 py-1 text-sm w-32">
							{#each allowedLevels(perm.min) as l}
								<option value={l}>{msg(`team_access_${l}`) ?? l}</option>
							{/each}
						</select>
					</label>
				{/each}
			</div>

			<h4 class="text-sm font-medium mt-4 mb-2">Standardnivåer för nya team</h4>
			<p class="text-xs text-neutral-500 mb-2">Nivå som tilldelas automatiskt skapade team vid första inloggning.</p>
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 mb-3">
				{#each defaultAccessConfig as cfg}
					<label class="flex items-center justify-between gap-2 text-sm">
						<span>{cfg.label}</span>
						<select bind:value={permForm[cfg.key]} class="border rounded px-2 py-1 text-sm w-32">
							{#each ['view', 'book', 'trusted', 'manager'] as l}
								<option value={l}>{msg(`team_access_${l}`) ?? l}</option>
							{/each}
						</select>
					</label>
				{/each}
				<label class="flex items-center justify-between gap-2 text-sm">
					<span>Standard godkännandenivå</span>
					<select bind:value={permForm['default_approval_level']} class="border rounded px-2 py-1 text-sm w-32">
						<option value="none">Ingen</option>
						<option value="low">Låg</option>
						<option value="high">Hög</option>
					</select>
				</label>
			</div>

			<div class="flex items-center gap-3">
				<button onclick={savePermissions} disabled={permSaving} class="text-sm bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50">
					{permSaving ? 'Sparar...' : 'Spara behörigheter'}
				</button>
				{#if permMessage}
					<span class="text-sm {permMessage.startsWith('Fel') ? 'text-red-600' : 'text-green-600'}">{permMessage}</span>
				{/if}
			</div>
		</section>

		<!-- Locations -->
		<section class="mb-6 border rounded-lg p-4">
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
		<section class="mb-6 border rounded-lg p-4">
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
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-2">Importera artiklar (CSV)</h3>
			<div class="flex flex-wrap items-center gap-2 mb-2">
				<input type="file" accept=".csv" onchange={handleFileSelect} class="text-sm file:mr-2 file:px-3 file:py-1 file:rounded file:border file:border-neutral-300 file:bg-white file:text-sm file:text-neutral-700 file:cursor-pointer hover:file:bg-neutral-50" />
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

		<!-- Group language -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-2">Språk</h3>
			<div class="flex items-center gap-3">
				<label class="text-sm text-neutral-700" for="group-language">Standardspråk för gruppen</label>
				<select id="group-language" bind:value={groupLanguage} class="border rounded px-2 py-1 text-sm">
					<option value="sv">Svenska</option>
					<option value="en">English</option>
				</select>
				<button onclick={saveGroupLanguage} class="text-sm bg-blue-700 text-white px-3 py-1 rounded">Spara</button>
				{#if groupLanguageMessage}<span class="text-sm text-green-600">{groupLanguageMessage}</span>{/if}
			</div>
		</section>

		<!-- Notifications -->
		<section class="mb-6 border rounded-lg p-4">
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
{/if}
