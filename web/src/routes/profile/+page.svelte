<script lang="ts">
	import { createApiClient, type Location, type Category, type GroupSettings, type Team, type TeamNotifSettings, type NotificationPrefs, type PerEventPrefs } from '$lib/api/client';
	import { browser } from '$app/environment';
	import CrudList from '$lib/components/CrudList.svelte';
	import type { PageData } from './$types';
	import { msg } from '$lib/msg';
	import * as m from '$lib/paraglide/messages.js';
	import { translateError } from '$lib/errors';

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
		} catch (e) { teamError = translateError(e); }
	}

	async function renameTeam(teamId: string) {
		if (!editingTeamName.trim()) return;
		teamError = '';
		try {
			const updated = await api.updateTeam(teamId, { name: editingTeamName.trim() });
			allTeams = allTeams.map(t => t.id === teamId ? { ...t, ...updated } : t);
			editingTeamId = null;
		} catch (e) { teamError = translateError(e); }
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
		} catch (e) { teamError = translateError(e); }
	}

	async function deleteTeam(teamId: string, teamName: string) {
		if (!confirm(m.page_profile_delete_team_confirm({ teamName }))) return;
		teamError = '';
		try {
			await api.deleteTeam(teamId);
			allTeams = allTeams.filter(t => t.id !== teamId);
		} catch (e) { teamError = translateError(e); }
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
			editMyError = translateError(e);
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
		let confirmMsg = m.page_profile_delete_image_confirm();
		if (totalRefs > 1) {
			const parts: string[] = [];
			if (img.own_group_count > 1) parts.push(m.page_profile_image_in_own_group({ count: String(img.own_group_count - 1) }));
			if (img.other_group_count > 0) parts.push(m.page_profile_image_in_other_groups({ count: String(img.other_group_count) }));
			confirmMsg = m.page_profile_image_delete_used({ refs: parts.join(' & ') });
		}
		if (!confirm(confirmMsg)) return;
		try {
			await api.deleteMyImage(img.id);
			myImages = myImages.filter(i => i.id !== img.id);
			if (expandedImageId === img.id) expandedImageId = null;
		} catch (e: any) {
			alert(translateError(e));
		}
	}

	$effect(() => {
		if (tab === 'profile') loadMyImages();
	});

	type Tab = 'profile' | 'teams' | 'group';
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
			const gs = data.groupSettings;
			settingsForm.notification_email_from = gs.notification_email_from;
			settingsForm.smtp_host = gs.smtp_host ?? '';
			settingsForm.smtp_port = gs.smtp_port ?? 587;
			settingsForm.smtp_tls = gs.smtp_tls ?? 'starttls';
			settingsForm.smtp_user = gs.smtp_user ?? '';
			useGroupSmtp = !!(gs.smtp_host);
		}
	});

	// CSV import
	let importFile = $state<File | null>(null);
	let importResult = $state<any>(null);
	let importLoading = $state(false);
	let importError = $state('');

	// Group notification settings
	let useGroupSmtp = $state(false);
	let settingsForm = $state({
		notification_email_from: '',
		smtp_host: '',
		smtp_port: 587,
		smtp_tls: 'starttls',
		smtp_user: '',
		smtp_key: ''
	});

	let settingsMessage = $state('');
	let settingsError = $state('');

	// Permission settings
	let permissionConfig = $derived([
		{ key: 'image_upload_role', label: m.page_profile_perm_upload_images(), min: 'view' },
		{ key: 'article_edit_role', label: m.page_profile_perm_edit_articles(), min: 'book' },
		{ key: 'issue_resolve_role', label: m.page_profile_perm_manage_issues(), min: 'book' },
		{ key: 'manager_notes_role', label: m.page_profile_perm_internal_notes(), min: 'trusted' },
	] as const);
	let defaultAccessConfig = $derived([
		{ key: 'default_access_unknown', label: m.page_profile_default_unknown() },
		{ key: 'default_access_troop', label: m.page_profile_default_new_troops() },
		{ key: 'default_access_role', label: m.page_profile_default_new_roles() },
	] as const);
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
			permMessage = m.common_saved();
			setTimeout(() => permMessage = '', 3000);
		} catch (e: any) {
			permMessage = m.page_profile_error_prefix() + translateError(e);
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
			importError = translateError(e);
		}
		importLoading = false;
	}

	// --- Notification preferences ---
	type EventRow = { key: string; label: () => string; locked?: boolean };

	// Broadcast (team/role) events for the group defaults table — personal events excluded.
	const allBookingEvents: EventRow[] = [
		{ key: 'booking_confirmed', label: m.notif_booking_confirmed },
		{ key: 'booking_cancelled', label: m.notif_booking_cancelled },
		{ key: 'booking_reminder', label: m.notif_booking_reminder },
		{ key: 'booking_overdue', label: m.notif_booking_overdue },
		{ key: 'booking_needs_approval', label: m.notif_booking_needs_approval },
		{ key: 'booking_submitted_no_approval', label: m.notif_booking_submitted_no_approval },
		{ key: 'booking_any_created', label: m.notif_booking_any_created },
	];
	const allIssueEvents: EventRow[] = [
		{ key: 'issue_created', label: m.notif_issue_created },
	];

	// Personal events: user is the named subject — simple on/off toggle.
	const personalEvents: EventRow[] = [
		{ key: 'booking_rejected', label: m.notif_booking_rejected },
		{ key: 'issue_assigned_to_me', label: m.notif_issue_assigned_to_me, locked: true },
		{ key: 'issue_resolved', label: m.notif_issue_resolved },
		{ key: 'issue_commented', label: m.notif_issue_commented },
	];

	// Team/role events: user notified as a team or role member — three-column radio.
	// Manager-only rows are hidden for non-managers.
	const teamEvents: EventRow[] = [
		{ key: 'booking_confirmed', label: m.notif_booking_confirmed },
		{ key: 'booking_cancelled', label: m.notif_booking_cancelled },
		{ key: 'booking_reminder', label: m.notif_booking_reminder },
		{ key: 'booking_overdue', label: m.notif_booking_overdue },
		{ key: 'booking_needs_approval', label: m.notif_booking_needs_approval },
		{ key: 'booking_submitted_no_approval', label: m.notif_booking_submitted_no_approval },
		{ key: 'booking_any_created', label: m.notif_booking_any_created },
		{ key: 'issue_created', label: m.notif_issue_created },
	];
	const managerOnlyKeys = new Set(['booking_needs_approval', 'booking_submitted_no_approval', 'booking_any_created', 'issue_created']);

	let notifPrefs = $state<NotificationPrefs | null>(null);
	$effect(() => { notifPrefs = data.notificationPrefs; });

	// channels from group settings, fallback to ['email']
	let notifChannels = $derived(data.groupSettings?.notification_channels ?? ['email']);

	function notifEnabled(key: string, ch: string): boolean {
		return notifPrefs?.[key]?.[ch]?.enabled ?? false;
	}

	// Three-column radio value for team/role events.
	// 'always' = explicit user true, 'never' = explicit user false, 'follow' = no user override.
	function teamEventRadio(key: string): 'always' | 'follow' | 'never' {
		const pref = notifPrefs?.[key]?.['email'];
		if (!pref || pref.source !== 'user') return 'follow';
		return pref.enabled ? 'always' : 'never';
	}

	async function toggleNotif(key: string, ch: string, value: boolean) {
		try {
			await api.updateNotificationPrefs({ [key]: { [ch]: value } });
			const result = await api.getNotificationPrefs();
			notifPrefs = result.prefs;
		} catch { /* ignore */ }
	}

	async function setTeamEventRadio(key: string, value: 'always' | 'follow' | 'never') {
		try {
			if (value === 'always') {
				await api.updateNotificationPrefs({ [key]: { email: true } });
			} else if (value === 'never') {
				await api.updateNotificationPrefs({ [key]: { email: false } });
			} else {
				// null removes the explicit user override, reverting to team/group/system default
				await api.updateNotificationPrefs({ [key]: { email: null } });
			}
			const result = await api.getNotificationPrefs();
			notifPrefs = result.prefs;
		} catch { /* ignore */ }
	}

	let notifRestoring = $state(false);
	async function restoreNotifDefaults() {
		notifRestoring = true;
		try {
			await api.resetNotificationPrefs();
			const result = await api.getNotificationPrefs();
			notifPrefs = result.prefs;
		} catch { /* ignore */ } finally {
			notifRestoring = false;
		}
	}

	// --- Teams tab state ---
	let selectedUserTeamId = $state<string | null>(null);
	let userTeamSettings = $state<TeamNotifSettings | null>(null);
	let userTeamSettingsLoading = $state(false);
	let userTeamSettingsError = $state('');
	let userTeamNameEdit = $state('');
	let userTeamNameSaving = $state(false);
	let userTeamNameMessage = $state('');
	let userTeamNotifSaving = $state(false);

	function switchToTeamsTab() {
		tab = 'teams';
		if (!selectedUserTeamId && user && user.teams.length > 0) {
			const first = user.teams[0];
			selectUserTeam(first.team_id, first.team_name);
		}
	}

	async function selectUserTeam(teamId: string, teamName: string) {
		selectedUserTeamId = teamId;
		userTeamNameEdit = teamName;
		userTeamSettings = null;
		userTeamSettingsError = '';
		userTeamSettingsLoading = true;
		try {
			userTeamSettings = await api.getTeamNotificationSettings(teamId);
		} catch {
			userTeamSettingsError = 'Kunde inte ladda avdelningsinställningar.';
		} finally {
			userTeamSettingsLoading = false;
		}
	}

	async function saveUserTeamName() {
		if (!selectedUserTeamId || !userTeamNameEdit.trim()) return;
		userTeamNameSaving = true;
		userTeamNameMessage = '';
		try {
			await api.updateTeamName(selectedUserTeamId, userTeamNameEdit.trim());
			userTeamNameMessage = 'Sparat!';
			setTimeout(() => userTeamNameMessage = '', 3000);
		} catch (e) {
			userTeamNameMessage = translateError(e);
		} finally {
			userTeamNameSaving = false;
		}
	}

	async function saveUserTeamNotifSettings(patch: Parameters<typeof api.updateTeamNotificationSettings>[1]) {
		if (!selectedUserTeamId) return;
		userTeamNotifSaving = true;
		try {
			await api.updateTeamNotificationSettings(selectedUserTeamId, patch);
			userTeamSettings = await api.getTeamNotificationSettings(selectedUserTeamId);
		} catch { /* ignore */ } finally {
			userTeamNotifSaving = false;
		}
	}

	// Team broadcast-channel events: shown in the team tab for broadcast toggles.
	// Visibility is per access_level: all teams see booking events; manager teams also see manager events.
	function teamTabEvents(accessLevel: string): EventRow[] {
		const base: EventRow[] = [
			{ key: 'booking_confirmed', label: m.notif_booking_confirmed },
			{ key: 'booking_cancelled', label: m.notif_booking_cancelled },
			{ key: 'booking_reminder', label: m.notif_booking_reminder },
			{ key: 'booking_overdue', label: m.notif_booking_overdue },
		];
		if (accessLevel === 'manager') {
			base.push(
				{ key: 'booking_needs_approval', label: m.notif_booking_needs_approval },
				{ key: 'booking_submitted_no_approval', label: m.notif_booking_submitted_no_approval },
				{ key: 'booking_any_created', label: m.notif_booking_any_created },
				{ key: 'issue_created', label: m.notif_issue_created },
			);
		}
		return base;
	}

	// --- GChat group settings state ---
	let gchatKeyJson = $state('');
	let gchatConnecting = $state(false);
	let gchatError = $state('');
	let gchatSpaces = $state<{ name: string; displayName: string }[]>([]);
	let gchatTeamMapperOpen = $state(false);
	let gchatTeams = $state<{ id: string; name: string; gchat_space_id: string }[]>([]);
	let gchatTeamSaving = $state<Record<string, boolean>>({});

	async function connectGChat() {
		gchatConnecting = true;
		gchatError = '';
		try {
			const result = await api.uploadGchatKey(gchatKeyJson);
			gchatSpaces = result.spaces;
			gchatKeyJson = '';
			groupSettings = await api.getGroupSettings();
		} catch {
			gchatError = m.page_profile_gchat_error_connect();
		} finally {
			gchatConnecting = false;
		}
	}

	async function disconnectGChat() {
		if (!confirm(m.page_profile_gchat_disconnect_confirm())) return;
		try {
			await api.deleteGchatKey();
			groupSettings = await api.getGroupSettings();
			gchatSpaces = [];
			gchatTeamMapperOpen = false;
		} catch { /* ignore */ }
	}

	async function loadGchatSpaces() {
		try {
			gchatSpaces = await api.listGchatSpaces();
		} catch { /* ignore */ }
	}

	async function loadGchatTeams() {
		try {
			const teams = await api.listTeams();
			gchatTeams = teams.map(t => ({ id: t.id, name: t.name, gchat_space_id: (t as any).gchat_space_id ?? '' }));
		} catch { /* ignore */ }
	}

	async function setGchatTeamSpace(teamId: string, spaceId: string) {
		gchatTeamSaving = { ...gchatTeamSaving, [teamId]: true };
		try {
			if (spaceId) {
				await api.setTeamGchatSpace(teamId, spaceId);
			} else {
				await api.clearTeamGchatSpace(teamId);
			}
			await loadGchatTeams();
		} catch { /* ignore */ } finally {
			gchatTeamSaving = { ...gchatTeamSaving, [teamId]: false };
		}
	}

	$effect(() => {
		if (mgr && groupSettings?.gchat_configured && gchatTeamMapperOpen && gchatTeams.length === 0) {
			loadGchatSpaces();
			loadGchatTeams();
		}
	});

	let testEmailSending = $state(false);
	let testEmailMessage = $state('');
	let testEmailError = $state(false);
	async function sendTestEmail() {
		testEmailSending = true;
		testEmailMessage = '';
		testEmailError = false;
		try {
			const result = await api.sendTestEmail();
			testEmailMessage = result.skipped
				? m.page_profile_notifs_send_test_skipped()
				: m.page_profile_notifs_send_test_sent();
		} catch {
			testEmailMessage = m.page_profile_notifs_send_test_error();
			testEmailError = true;
		} finally {
			testEmailSending = false;
			setTimeout(() => testEmailMessage = '', 5000);
		}
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
			languageMessage = m.page_profile_error_prefix() + translateError(e);
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
			flash(v => groupLanguageMessage = v, m.common_saved());
		} catch (e: any) {
			groupLanguageMessage = m.page_profile_error_prefix() + translateError(e);
		}
	}

	// --- Group notification settings ---
	async function saveSettings() {
		settingsError = '';
		try {
			const payload: Record<string, any> = {
				notification_email_from: useGroupSmtp ? settingsForm.notification_email_from : '',
				smtp_host: useGroupSmtp ? settingsForm.smtp_host : '',
				smtp_port: useGroupSmtp ? settingsForm.smtp_port : 587,
				smtp_tls: useGroupSmtp ? settingsForm.smtp_tls : 'starttls',
				smtp_user: useGroupSmtp ? settingsForm.smtp_user : '',
			};
			if (useGroupSmtp && settingsForm.smtp_key) {
				payload.smtp_key = settingsForm.smtp_key;
			} else if (!useGroupSmtp) {
				payload.smtp_key = '';  // clear key when disabling group SMTP
			}
			groupSettings = await api.updateGroupSettings(payload);
			settingsForm.smtp_key = '';
			flash(v => settingsMessage = v, m.page_profile_settings_saved());
		} catch (e: any) {
			settingsError = translateError(e);
		}
	}

	// --- Group notification defaults ---
	let groupNotifDefaults = $state<Record<string, PerEventPrefs>>({});
	let groupSysDefaults = $state<Record<string, PerEventPrefs>>({});
	let groupDefaultGruppkanalChannels = $state<string[]>([]);
	let groupNotifDefaultsMessage = $state('');
	let groupNotifDefaultsError = $state(false);

	async function loadGroupNotifDefaults() {
		try {
			const result = await api.getGroupNotificationDefaults();
			groupNotifDefaults = result.defaults ?? {};
			groupSysDefaults = result.system_defaults ?? {};
			groupDefaultGruppkanalChannels = result.default_gruppkanal_channels ?? [];
		} catch {}
	}

	$effect(() => {
		if (mgr) loadGroupNotifDefaults();
	});

	async function saveGroupDefaults(patch: Partial<{ defaults: Record<string, PerEventPrefs>; default_gruppkanal_channels: string[] }>) {
		const payload = {
			defaults: patch.defaults ?? groupNotifDefaults,
			default_gruppkanal_channels: patch.default_gruppkanal_channels ?? groupDefaultGruppkanalChannels,
		};
		try {
			await api.updateGroupNotificationDefaults(payload);
			groupNotifDefaults = payload.defaults;
			groupDefaultGruppkanalChannels = payload.default_gruppkanal_channels;
			groupNotifDefaultsError = false;
			flash(v => groupNotifDefaultsMessage = v, m.common_saved());
		} catch (e: any) {
			groupNotifDefaultsError = true;
			groupNotifDefaultsMessage = m.page_profile_error_prefix() + translateError(e);
		}
	}

	let forceDefaultsRunning = $state(false);
	async function forceNotificationDefaults() {
		if (!confirm(m.page_profile_notif_force_defaults_confirm_v2())) return;
		forceDefaultsRunning = true;
		try {
			const result = await api.forceGroupNotificationDefaults();
			flash(v => groupNotifDefaultsMessage = v, m.page_profile_notif_force_defaults_done_v2({ user_count: String(result.reset_user_count), team_count: String(result.reset_team_count) }));
			groupNotifDefaultsError = false;
		} catch (e: any) {
			groupNotifDefaultsError = true;
			groupNotifDefaultsMessage = m.page_profile_error_prefix() + translateError(e);
		} finally {
			forceDefaultsRunning = false;
		}
	}

	// --- Team notification settings ---
	let teamNotifSettings = $state<TeamNotifSettings | null>(null);
	let teamNotifSaving = $state(false);
	let teamNotifMessage = $state('');
	let teamNotifError = $state(false);

	$effect(() => {
		if (selectedTeamId && mgr) {
			teamNotifSettings = null;
			api.getTeamNotificationSettings(selectedTeamId).then(s => { teamNotifSettings = s; }).catch(() => {});
		} else {
			teamNotifSettings = null;
		}
	});

	async function saveTeamNotifSettings(patch: Partial<Pick<TeamNotifSettings, 'notification_email' | 'notification_prefs' | 'gruppkanal_channels'>>) {
		if (!selectedTeamId) return;
		teamNotifSaving = true;
		try {
			await api.updateTeamNotificationSettings(selectedTeamId, patch);
			teamNotifSettings = { ...teamNotifSettings!, ...patch };
			teamNotifError = false;
			flash(v => teamNotifMessage = v, m.common_saved());
		} catch (e: any) {
			teamNotifError = true;
			teamNotifMessage = m.page_profile_error_prefix() + translateError(e);
		} finally {
			teamNotifSaving = false;
		}
	}

	async function toggleTeamNotifPref(key: string, ch: string, value: boolean) {
		if (!teamNotifSettings) return;
		const updated = {
			...teamNotifSettings.notification_prefs,
			[key]: { ...(teamNotifSettings.notification_prefs[key] ?? {}), [ch]: value }
		};
		await saveTeamNotifSettings({ notification_prefs: updated });
	}
</script>

{#if !user}
	<div class="max-w-5xl mx-auto px-4 py-8">
		<p class="text-neutral-500">{m.btn_loading()}</p>
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
		>{m.page_profile_tab_profile()}</button>
		{#if user && user.teams.length > 0}
			<button
				onclick={() => switchToTeamsTab()}
				class="px-3 py-2 text-sm -mb-px {tab === 'teams' ? 'border-b-2 border-blue-700 font-medium text-blue-700' : 'text-neutral-500'}"
			>{m.page_profile_tab_teams()}</button>
		{/if}
		{#if mgr}
			<button
				onclick={() => tab = 'group'}
				class="px-3 py-2 text-sm -mb-px {tab === 'group' ? 'border-b-2 border-blue-700 font-medium text-blue-700' : 'text-neutral-500'}"
			>{m.page_profile_tab_group()}</button>
		{/if}
	</div>

	<!-- Profile tab -->
	{#if tab === 'profile'}
		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-3">{m.page_profile_permissions_heading()}</h2>

			{#if user.teams.length === 0}
				<p class="text-sm text-neutral-500">{m.page_profile_no_teams()}</p>
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
										<span class="text-neutral-400">{team.team_type === 'troop' ? m.team_type_troop() : m.team_type_role()}</span>
									</span>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-3">{m.page_profile_settings_heading()}</h2>
			<div class="flex items-center gap-3">
				<label class="text-sm text-neutral-700" for="user-language">{m.page_profile_language_label()}</label>
				<select id="user-language" bind:value={userLanguage} class="border rounded px-2 py-1 text-sm">
					<option value="sv">Svenska{groupLanguage !== 'sv' ? '' : ` ${m.page_profile_language_group_default()}`}</option>
					<option value="en">English{groupLanguage !== 'en' ? '' : ` ${m.page_profile_language_group_default()}`}</option>
				</select>
				<button onclick={saveLanguage} disabled={languageSaving} class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">
					{languageSaving ? m.btn_saving() : m.btn_save()}
				</button>
				{#if languageMessage}<span class="text-sm text-green-600">{languageMessage}</span>{/if}
			</div>
			<p class="text-xs text-neutral-400 mt-2">{m.page_profile_language_hint()}</p>
		</section>

		<!-- Notification preferences -->
		{#if notifPrefs}
		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-4">{m.page_profile_notifs_heading()}</h2>

			<!-- Personal events: simple on/off -->
			<h3 class="text-sm font-semibold text-neutral-600 mb-2">{m.page_profile_notifs_personal_heading()}</h3>
			<p class="text-xs text-neutral-500 mb-3">{m.page_profile_notifs_personal_help()}</p>
			<table class="w-full text-sm border-collapse mb-6">
				<thead>
					<tr>
						<th class="text-left pb-1 pr-3 font-normal text-xs text-neutral-400 w-full"></th>
						<th class="pb-1 px-3 font-normal text-xs text-neutral-400 whitespace-nowrap text-center">{m.page_profile_notifs_channel_email()}</th>
					</tr>
				</thead>
				<tbody>
					{#each personalEvents as row}
					<tr class="border-t border-neutral-100">
						<td class="py-1.5 pr-3 w-full">
							<span>{row.label()}</span>
							{#if row.locked}
								<span class="text-xs text-neutral-400 ml-1" title={m.page_profile_notifs_assigned_locked()}>🔒</span>
							{/if}
						</td>
						<td class="py-1.5 px-3 text-center">
							{#if row.locked}
								<input type="checkbox" checked disabled class="h-4 w-4 accent-blue-700 opacity-50 cursor-not-allowed" />
							{:else}
								<input
									type="checkbox"
									checked={notifEnabled(row.key, 'email')}
									onchange={(e) => toggleNotif(row.key, 'email', e.currentTarget.checked)}
									class="h-4 w-4 accent-blue-700"
								/>
							{/if}
						</td>
					</tr>
					{/each}
				</tbody>
			</table>

			<!-- Team/role events: three-column radio -->
			<div class="flex items-center justify-between mb-2">
				<h3 class="text-sm font-semibold text-neutral-600">{m.page_profile_notifs_team_heading()}</h3>
				{#if user && user.teams.length > 0}
					<button
						onclick={() => switchToTeamsTab()}
						class="text-xs text-blue-700 underline"
					>{m.page_profile_notifs_manage_teams_link()}</button>
				{/if}
			</div>
			<!-- Desktop header (hidden on mobile) -->
			<div class="hidden sm:flex text-xs text-neutral-500 border-b border-neutral-200 pb-1 mb-1">
				<span class="flex-1 pr-3"></span>
				<span class="w-28 text-center leading-tight">{m.page_profile_notifs_always_email()}</span>
				<span class="w-28 text-center leading-tight">{m.page_profile_notifs_follow_team()}</span>
				<span class="w-28 text-center leading-tight">{m.page_profile_notifs_no_email()}</span>
			</div>
			<div class="space-y-0">
				{#each teamEvents as row}
					{#if !managerOnlyKeys.has(row.key) || mgr}
					<div class="border-t border-neutral-100 py-2 flex flex-col sm:flex-row sm:items-center gap-1.5 sm:gap-0">
						<span class="text-sm flex-1 pr-3">{row.label()}</span>
						<div class="flex gap-0">
							{#each [
								{ val: 'always', short: m.page_profile_notifs_always_short() },
								{ val: 'follow', short: m.page_profile_notifs_follow_short() },
								{ val: 'never', short: m.page_profile_notifs_no_email_short() },
							] as opt}
							<label class="flex items-center gap-1 sm:w-28 sm:justify-center cursor-pointer pr-4 sm:pr-0">
								<input
									type="radio"
									name="notif-{row.key}"
									value={opt.val}
									checked={teamEventRadio(row.key) === opt.val}
									onchange={() => setTeamEventRadio(row.key, opt.val as 'always' | 'follow' | 'never')}
									class="h-4 w-4 accent-blue-700 shrink-0"
								/>
								<span class="text-xs text-neutral-600 sm:hidden">{opt.short}</span>
							</label>
							{/each}
						</div>
					</div>
					{/if}
				{/each}
			</div>

			<div class="mt-3">
				<button onclick={restoreNotifDefaults} disabled={notifRestoring} class="text-sm text-neutral-500 underline disabled:opacity-50">
					{m.page_profile_notifs_restore()}
				</button>
			</div>
		</section>
		{/if}

		<section class="mb-6 border rounded-lg p-4">
			<h2 class="font-medium mb-3">{m.page_profile_images_heading()}</h2>

		{#if !myImagesLoaded}
			<p class="text-sm text-neutral-400">{m.btn_loading()}</p>
		{:else if myImages.length === 0}
			<p class="text-sm text-neutral-500">{m.page_profile_images_empty()}</p>
		{:else}
			<p class="text-xs text-neutral-400 mb-2">{myImages.length} {myImages.length === 1 ? m.page_profile_image_singular() : m.page_profile_image_plural()}</p>
			{#if editingMyImage}
				<div class="border rounded-lg p-4 bg-white space-y-3 mb-4">
					<div class="flex items-start gap-4">
						<img src="/api/v0/images/{editingMyImage.file_id}_thumb.webp" alt={editingMyImage.title} class="h-32 rounded object-contain shrink-0" />
						<div class="flex-1 space-y-3 min-w-0">
							<label class="block">
								<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_image_title_label()}</span>
								<input type="text" bind:value={editMyTitle} class="border rounded px-2 py-1.5 text-sm w-full" />
							</label>
							<label class="block">
								<span class="text-sm text-neutral-600 block mb-1">{m.lbl_description()}</span>
								<textarea bind:value={editMyDescription} rows={2} class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
							</label>
							<label class="block">
								<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_image_photographer_label()}</span>
								<input type="text" bind:value={editMyAttribution} class="border rounded px-2 py-1.5 text-sm w-full" />
							</label>
							<label class="flex items-center gap-2 text-sm">
								<input type="checkbox" bind:checked={editMyShared} />
								{m.page_profile_image_share_label()}
							</label>
						</div>
					</div>
					{#if editMyError}
						<p class="text-xs text-red-600">{editMyError}</p>
					{/if}
					<div class="flex gap-2">
						<button type="button" onclick={saveEditMyImage} disabled={editMySaving} class="text-sm bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50">{editMySaving ? m.btn_saving() : m.btn_save()}</button>
						<button type="button" onclick={() => editingMyImage = null} class="text-sm text-neutral-500 underline">{m.btn_cancel()}</button>
					</div>
				</div>
			{/if}
			<div class="flex flex-wrap gap-3">
				{#each myImages as img}
					<div class="border rounded overflow-hidden w-[calc(33.333%-0.5rem)]">
						<button type="button" onclick={() => openFullscreen(img)} class="w-full cursor-zoom-in">
							<img
								src="/api/v0/images/{img.file_id}_thumb.webp"
								alt={img.title || m.page_profile_image_no_title()}
								class="w-full h-[160px] rounded-t object-contain bg-neutral-50"
								loading="lazy"
							/>
						</button>
						<div class="px-2 py-1.5">
							<p class="text-xs font-medium truncate">{img.title || m.page_profile_image_no_title()}</p>
							{#if img.shared}
								<span class="text-[10px] bg-blue-100 text-blue-700 px-1 rounded">{m.page_profile_image_shared_badge()}</span>
							{/if}
							<div class="flex gap-1.5 mt-1">
								<button type="button" onclick={() => toggleImageDetail(img)} class="text-[11px] text-blue-700 border border-blue-200 bg-blue-50 rounded px-1.5 py-0.5 hover:bg-blue-100">
									{expandedImageId === img.id ? m.page_profile_btn_hide() : m.page_profile_btn_details()}
								</button>
								<button type="button" onclick={() => startEditMyImage(img)} class="text-[11px] text-blue-700 border border-blue-200 bg-blue-50 rounded px-1.5 py-0.5 hover:bg-blue-100">{m.btn_edit()}</button>
							</div>
						</div>

						{#if expandedImageId === img.id}
							<div class="border-t px-2 py-2 space-y-2 bg-neutral-50">

								{#if img.description}
									<p class="text-xs text-neutral-600">{img.description}</p>
								{/if}

								<p class="text-[10px] text-neutral-400">
									{img.format === 'landscape' ? m.page_profile_image_orientation_landscape() : img.format === 'portrait' ? m.page_profile_image_orientation_portrait() : m.page_profile_image_orientation_square()}
									· {new Date(img.created_at).toLocaleDateString('sv')}
								</p>

								{#if expandedArticles.length > 0}
									<div>
										<span class="text-[10px] font-medium text-neutral-500">{m.page_profile_image_used_on()}</span>
										{#each expandedArticles as a}
											<a href="/articles/{a.article_id}" class="block text-[10px] text-blue-700 hover:underline">{a.commercial_name} — {a.location_name}</a>
										{/each}
									</div>
								{:else}
									<p class="text-[10px] text-neutral-400">{m.page_profile_image_not_linked()}</p>
								{/if}

								{#if img.other_group_count > 0}
									<p class="text-[10px] text-neutral-400">{m.page_profile_image_other_groups({ count: String(img.other_group_count) })}</p>
								{/if}

								<button type="button" onclick={() => deleteMyImage(img)} class="text-[10px] text-red-600 hover:underline">{m.page_profile_btn_delete_image()}</button>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
		</section>

		<form method="POST" action="/auth/signout" class="mt-4">
			<button type="submit" class="text-sm text-red-600 hover:underline">{m.page_profile_btn_logout()}</button>
		</form>

	<!-- Teams tab -->
	{:else if tab === 'teams'}
		{#if !user || user.teams.length === 0}
			<p class="text-sm text-neutral-500">{m.page_profile_teams_no_teams()}</p>
		{:else}
			<!-- Team picker -->
			<div class="flex flex-wrap gap-2 mb-6">
				{#each user.teams as membership}
					<button
						onclick={() => selectUserTeam(membership.team_id, membership.team_name)}
						class="px-3 py-1.5 text-sm rounded-full border {selectedUserTeamId === membership.team_id ? 'bg-blue-700 text-white border-blue-700' : 'bg-white text-neutral-700 border-neutral-300 hover:border-blue-400'}"
					>
						{membership.team_name}
						<span class="ml-1 opacity-60 text-xs">{membership.team_type === 'troop' ? m.team_type_troop() : m.team_type_role()}</span>
					</button>
				{/each}
			</div>

			{#if selectedUserTeamId}
				{@const selectedMembership = user.teams.find(t => t.team_id === selectedUserTeamId)}
				<div class="border rounded-lg divide-y">

					<!-- Team name -->
					<div class="p-4">
						<label class="block" for="user-team-name">
							<span class="text-sm font-medium text-neutral-700 block mb-1">{m.page_profile_team_name_label()}</span>
						</label>
						<div class="flex gap-2 items-center">
							<input
								id="user-team-name"
								type="text"
								bind:value={userTeamNameEdit}
								class="border rounded px-2 py-1 text-sm w-full max-w-xs"
							/>
							<button
								onclick={saveUserTeamName}
								disabled={userTeamNameSaving}
								class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50"
							>{userTeamNameSaving ? m.btn_saving() : m.btn_save()}</button>
							{#if userTeamNameMessage}
								<span class="text-sm text-green-600">{userTeamNameMessage}</span>
							{/if}
						</div>
					</div>

					<!-- Notification settings -->
					<div class="p-4">
						<h3 class="text-sm font-semibold text-neutral-700 mb-3">{m.page_profile_teams_notif_heading()}</h3>

						{#if userTeamSettingsLoading}
							<p class="text-sm text-neutral-400">{m.btn_loading()}</p>
						{:else if userTeamSettingsError}
							<p class="text-sm text-red-600">{userTeamSettingsError}</p>
						{:else if userTeamSettings}
							{@const effectiveGruppkanal = userTeamSettings.gruppkanal_channels ?? userTeamSettings.default_gruppkanal_channels}
							{@const hasGroupEmail = !!userTeamSettings.notification_email}
							{@const hasGChat = !!userTeamSettings.gchat_space_id}
							{@const availableChannels = [
								...(hasGroupEmail ? ['email'] : []),
								...(hasGChat ? ['gchat'] : []),
							]}

							<!-- Gruppkanal composition -->
							<div class="mb-5">
								<h4 class="text-sm font-medium text-neutral-700 mb-1">{m.page_profile_teams_gruppkanal_heading()}</h4>
								<p class="text-xs text-neutral-400 mb-3">{m.page_profile_teams_gruppkanal_help()}</p>

								{#if availableChannels.length === 0}
									<p class="text-sm text-neutral-400 italic">{m.page_profile_teams_gruppkanal_no_channels()}</p>
								{:else}
									{#each availableChannels as ch}
										{@const isInherited = userTeamSettings.gruppkanal_channels === null}
										{@const checked = effectiveGruppkanal.includes(ch)}
										<label class="flex items-center gap-2 mb-1 cursor-pointer">
											<input
												type="checkbox"
												{checked}
												onchange={(e) => {
													const current = userTeamSettings!.gruppkanal_channels ?? [...userTeamSettings!.default_gruppkanal_channels];
													const updated = e.currentTarget.checked
														? [...current, ch]
														: current.filter(c => c !== ch);
													saveUserTeamNotifSettings({ gruppkanal_channels: updated });
												}}
												class="h-4 w-4 accent-blue-700"
											/>
											<span class="text-sm text-neutral-700">
												{ch === 'email' ? m.page_profile_teams_gruppkanal_channel_email() : m.page_profile_teams_gruppkanal_channel_gchat()}
												{#if ch === 'email' && userTeamSettings.notification_email}
													<span class="text-neutral-400 text-xs ml-1">({userTeamSettings.notification_email})</span>
												{/if}
												{#if isInherited}
													<span class="text-neutral-400 text-xs ml-1">{m.page_profile_teams_gruppkanal_inherited()}</span>
												{/if}
											</span>
										</label>
									{/each}
								{/if}
							</div>

							<!-- Per-event table: Personlig e-post radio + Gruppkanal checkbox -->
							{@const tabEvents = teamTabEvents(selectedMembership?.access_level ?? 'book')}
							{@const showGruppkanalCol = effectiveGruppkanal.length > 0}
							<table class="w-full text-sm border-collapse">
								<thead>
									<tr>
										<th class="text-left py-1 pr-3 font-normal text-neutral-500 w-full"></th>
										<th class="text-center py-1 px-2 font-normal text-neutral-500 whitespace-nowrap">{m.page_profile_teams_notif_col_personal()}</th>
										{#if showGruppkanalCol}
											<th class="text-center py-1 px-3 font-normal text-neutral-500 whitespace-nowrap">{m.page_profile_teams_notif_col_gruppkanal()}</th>
										{/if}
									</tr>
								</thead>
								<tbody>
									{#each tabEvents as row}
									{@const ep = userTeamSettings.notification_prefs?.[row.key]}
									{@const policy = ep?.personal_email_policy ?? ''}
									{@const grEnabled = ep?.gruppkanal ?? null}
									{@const isInherited = ep === undefined || ep === null}
									<tr class="border-t border-neutral-100">
										<td class="py-2 pr-3">{row.label()}</td>
										<!-- Personal email: compact 3-option radio -->
										<td class="py-2 px-2">
											<div class="flex gap-1 justify-center">
												{#each [['always', m.page_profile_teams_notif_personal_always()], ['if_no_broadcast', m.page_profile_teams_notif_personal_if_no_broadcast()], ['never', m.page_profile_teams_notif_personal_never()]] as [val, label]}
													<button
														onclick={() => {
															const prefs = { ...userTeamSettings!.notification_prefs };
															prefs[row.key] = { ...(prefs[row.key] ?? {}), personal_email_policy: val as 'always' | 'if_no_broadcast' | 'never' };
															saveUserTeamNotifSettings({ notification_prefs: prefs });
														}}
														class="px-2 py-0.5 text-xs rounded border {(policy || 'if_no_broadcast') === val ? 'bg-blue-700 text-white border-blue-700' : 'bg-white text-neutral-600 border-neutral-300 hover:border-blue-400'}
														{val === 'if_no_broadcast' && effectiveGruppkanal.length === 0 ? 'opacity-40' : ''}"
														title={val === 'if_no_broadcast' && effectiveGruppkanal.length === 0 ? m.page_profile_teams_notif_personal_if_no_broadcast_dimmed_tooltip() : ''}
													>{label}</button>
												{/each}
												{#if policy !== '' && policy !== 'if_no_broadcast'}
													<span class="text-xs text-neutral-400 self-center ml-1">{m.page_profile_teams_notif_inherited()}</span>
												{/if}
											</div>
										</td>
										<!-- Gruppkanal checkbox -->
										{#if showGruppkanalCol}
										<td class="py-2 px-3 text-center">
											<input
												type="checkbox"
												checked={grEnabled ?? true}
												onchange={(e) => {
													const prefs = { ...userTeamSettings!.notification_prefs };
													prefs[row.key] = { ...(prefs[row.key] ?? {}), gruppkanal: e.currentTarget.checked };
													saveUserTeamNotifSettings({ notification_prefs: prefs });
												}}
												class="h-4 w-4 accent-blue-700"
											/>
											{#if grEnabled === null}
												<span class="block text-xs text-neutral-400">{m.page_profile_teams_notif_inherited()}</span>
											{/if}
										</td>
										{/if}
									</tr>
									{/each}
								</tbody>
							</table>
						{/if}
					</div>

					<!-- Integrations (read-only) -->
					<div class="p-4">
						<h3 class="text-sm font-semibold text-neutral-700 mb-2">{m.page_profile_teams_integrations_heading()}</h3>
						{#if userTeamSettings}
							<div class="text-sm text-neutral-600 space-y-1">
								<div>
									<span class="text-neutral-400">{m.page_profile_teams_notif_email_label()}:</span>
									{userTeamSettings.notification_email || '–'}
								</div>
								<div>
									<span class="text-neutral-400">{m.page_profile_teams_gchat_space_label()}:</span>
									{#if userTeamSettings.gchat_space_id}
										{userTeamSettings.gchat_space_id}
									{:else}
										{m.page_profile_teams_gchat_space_none()}
									{/if}
								</div>
								{#if !mgr}
									<p class="text-xs text-neutral-400 mt-1">{m.page_profile_teams_gchat_space_managed_by_manager()}</p>
								{/if}
							</div>
						{/if}
					</div>

				</div>
			{/if}
		{/if}

	<!-- Group settings tab (manager only) -->
	{:else if tab === 'group'}

		<!-- Teams -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-1">{m.page_profile_teams_heading()}</h3>
			<p class="text-xs text-neutral-500 mb-3">{m.page_profile_teams_help()}</p>
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
								<p class="text-xs text-neutral-400 italic px-2 py-1">{m.page_profile_teams_empty()}</p>
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
								<button onclick={() => renameTeam(team.id)} class="text-sm text-blue-700 underline">{m.btn_save()}</button>
								<button onclick={() => editingTeamId = null} class="text-sm text-neutral-500 underline">{m.btn_cancel()}</button>
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
									<span class="text-neutral-600">{m.page_profile_access_level_label()}</span>
									<select
										value={team.access_level}
										onchange={(e) => changeTeamLevel(team.id, e.currentTarget.value)}
										class="border rounded px-2 py-1 text-sm"
										aria-label={m.page_profile_change_access_aria()}
									>
										{#each accessLevels as l}
											<option value={l}>{msg(`team_access_${l}`) ?? l}</option>
										{/each}
									</select>
								</label>
								<button onclick={() => { editingTeamId = team.id; editingTeamName = team.name; }} class="text-sm text-blue-700 underline">{m.page_profile_btn_rename()}</button>
								<button onclick={() => deleteTeam(team.id, team.name)} class="text-sm text-red-600 underline">{m.btn_delete()}</button>
							</div>
						{/if}

						<!-- Team notification settings -->
						{#if editingTeamId !== team.id}
							<div class="border-t pt-3 mt-1">
								<h4 class="text-sm font-medium mb-2">{m.page_profile_team_notif_heading()}</h4>
								{#if teamNotifSettings}
									{#if teamNotifMessage}
										<p class="text-sm mb-2 {teamNotifError ? 'text-red-600' : 'text-green-600'}">{teamNotifMessage}</p>
									{/if}
									<div class="space-y-3">
										<label class="flex flex-col gap-0.5">
											<span class="text-xs text-neutral-500">{m.page_profile_team_notif_broadcast_email()}</span>
											<div class="flex gap-2">
												<input
													type="email"
													value={teamNotifSettings.notification_email}
													oninput={(e) => { teamNotifSettings = { ...teamNotifSettings!, notification_email: e.currentTarget.value }; }}
													placeholder="team@example.com"
													class="border rounded px-2 py-1 text-sm w-full max-w-sm"
												/>
												<button
													onclick={() => saveTeamNotifSettings({ notification_email: teamNotifSettings!.notification_email })}
													disabled={teamNotifSaving}
													class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50"
												>{m.btn_save()}</button>
											</div>
											<span class="text-xs text-neutral-400">{m.page_profile_team_notif_broadcast_email_help()}</span>
										</label>

										<div>
											<p class="text-xs text-neutral-500 mb-1.5">{m.page_profile_team_notif_prefs_help()}</p>
											<table class="w-full text-sm border-collapse">
												<thead>
													<tr>
														<th class="text-left py-1 pr-3 font-normal text-neutral-500 w-full"></th>
														<th class="text-center py-1 px-2 font-normal text-neutral-500 whitespace-nowrap text-xs">{m.page_profile_teams_notif_col_gruppkanal()}</th>
													</tr>
												</thead>
												<tbody>
													<tr>
														<td colspan="2" class="py-1 text-xs font-semibold text-neutral-400 uppercase tracking-wide">{m.page_profile_notifs_bookings_group()}</td>
													</tr>
													{#each allBookingEvents as row}
														<tr class="border-t border-neutral-100">
															<td class="py-1.5 pr-3 text-sm">{row.label()}</td>
															<td class="py-1.5 px-2 text-center">
																<input
																	type="checkbox"
																	checked={teamNotifSettings.notification_prefs[row.key]?.gruppkanal ?? true}
																	onchange={(e) => {
																		const prefs = { ...teamNotifSettings!.notification_prefs };
																		prefs[row.key] = { ...(prefs[row.key] ?? {}), gruppkanal: e.currentTarget.checked };
																		saveTeamNotifSettings({ notification_prefs: prefs });
																	}}
																	class="h-4 w-4 accent-blue-700"
																/>
															</td>
														</tr>
													{/each}
													<tr>
														<td colspan="2" class="pt-2 pb-1 text-xs font-semibold text-neutral-400 uppercase tracking-wide">{m.page_profile_notifs_issues_group()}</td>
													</tr>
													{#each allIssueEvents as row}
														<tr class="border-t border-neutral-100">
															<td class="py-1.5 pr-3 text-sm">{row.label()}</td>
															<td class="py-1.5 px-2 text-center">
																<input
																	type="checkbox"
																	checked={teamNotifSettings.notification_prefs[row.key]?.gruppkanal ?? true}
																	onchange={(e) => {
																		const prefs = { ...teamNotifSettings!.notification_prefs };
																		prefs[row.key] = { ...(prefs[row.key] ?? {}), gruppkanal: e.currentTarget.checked };
																		saveTeamNotifSettings({ notification_prefs: prefs });
																	}}
																	class="h-4 w-4 accent-blue-700"
																/>
															</td>
														</tr>
													{/each}
												</tbody>
											</table>
										</div>
									</div>
								{:else}
									<p class="text-xs text-neutral-400">{m.btn_loading()}</p>
								{/if}
							</div>
						{/if}
					</div>
				{/if}
			{/if}

			{#if showAddTeam}
				<div class="border rounded-lg p-3 space-y-2 mb-2">
					<div class="flex flex-wrap gap-2">
						<label class="flex flex-col gap-0.5 flex-1 min-w-[150px]">
							<span class="text-xs text-neutral-500">{m.page_profile_team_name_label()}</span>
							<input type="text" bind:value={newTeam.name} placeholder={m.page_profile_team_name_placeholder()} class="border rounded px-2 py-1 text-sm" />
						</label>
						<label class="flex flex-col gap-0.5">
							<span class="text-xs text-neutral-500">{m.page_profile_team_type_label()}</span>
							<select bind:value={newTeam.type} class="border rounded px-2 py-1 text-sm">
								<option value="troop">{m.page_profile_team_type_troop()}</option>
								<option value="role">{m.page_profile_team_type_role()}</option>
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
							<span class="text-xs text-neutral-500">{m.page_profile_scoutnet_id()}</span>
							<select bind:value={newTeam.claim_scope} class="border rounded px-2 py-1 text-sm">
								<option value="troop">{m.page_profile_team_type_troop()}</option>
								<option value="group">{m.page_profile_team_type_group_role()}</option>
							</select>
						</label>
						<label class="flex flex-col gap-0.5 flex-1 min-w-[100px]">
							<span class="text-xs text-neutral-500">{newTeam.claim_scope === 'troop' ? m.page_profile_troop_id() : m.page_profile_role_name()} <span class="text-neutral-400">{m.page_profile_from_scoutnet()}</span></span>
							<input type="text" bind:value={newTeam.claim_id} required placeholder={newTeam.claim_scope === 'troop' ? 'T.ex. 17443' : 'T.ex. it_manager'} class="border rounded px-2 py-1 text-sm" />
						</label>
					</div>
					<p class="text-xs text-neutral-400">{m.page_profile_scoutnet_id_help()}</p>
					<div class="flex gap-2">
						<button onclick={addTeam} class="text-sm bg-blue-700 text-white px-3 py-1 rounded">{m.btn_add()}</button>
						<button onclick={() => showAddTeam = false} class="text-sm text-neutral-500 underline">{m.btn_cancel()}</button>
					</div>
				</div>
			{:else}
				<button onclick={() => showAddTeam = true} class="text-sm text-blue-700 underline">{m.page_profile_btn_add_team()}</button>
			{/if}
		</section>

		<!-- Permissions -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-1">{m.page_profile_access_section()}</h3>
			<p class="text-xs text-neutral-500 mb-3">{m.page_profile_access_help()}</p>
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

			<h4 class="text-sm font-medium mt-4 mb-2">{m.page_profile_default_levels_heading()}</h4>
			<p class="text-xs text-neutral-500 mb-2">{m.page_profile_default_levels_help()}</p>
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
					<span>{m.page_profile_default_approval()}</span>
					<select bind:value={permForm['default_approval_level']} class="border rounded px-2 py-1 text-sm w-32">
						<option value="none">{m.article_approval_none()}</option>
						<option value="low">{m.article_approval_low()}</option>
						<option value="high">{m.article_approval_high()}</option>
					</select>
				</label>
			</div>

			<div class="flex items-center gap-3">
				<button onclick={savePermissions} disabled={permSaving} class="text-sm bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50">
					{permSaving ? m.btn_saving() : m.page_profile_btn_save_permissions()}
				</button>
				{#if permMessage}
					<span class="text-sm {permMessage.startsWith('Fel') ? 'text-red-600' : 'text-green-600'}">{permMessage}</span>
				{/if}
			</div>
		</section>

		<!-- Locations -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-2">{m.page_profile_locations_heading()}</h3>
			<CrudList
				bind:items={locations}
				label={m.page_profile_location_label()}
				placeholder={m.page_profile_location_placeholder()}
				onCreate={(name) => api.createLocation({ name, sort_order: locations.length + 1 })}
				onUpdate={(id, name) => api.updateLocation(id, { name })}
				onDelete={(id) => api.deleteLocation(id)}
			/>
		</section>

		<!-- Categories -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-2">{m.page_profile_categories_heading()}</h3>
			<CrudList
				bind:items={categories}
				label={m.page_profile_category_label()}
				placeholder={m.page_profile_category_placeholder()}
				onCreate={(name) => api.createCategory({ name, sort_order: categories.length + 1 })}
				onUpdate={(id, name) => api.updateCategory(id, { name })}
				onDelete={(id) => api.deleteCategory(id)}
			/>
		</section>

		<!-- CSV Import -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-2">{m.page_profile_import_heading()}</h3>
			<div class="flex flex-wrap items-center gap-2 mb-2">
				<input type="file" accept=".csv" onchange={handleFileSelect} class="text-sm file:mr-2 file:px-3 file:py-1 file:rounded file:border file:border-neutral-300 file:bg-white file:text-sm file:text-neutral-700 file:cursor-pointer hover:file:bg-neutral-50" />
				<button onclick={runImport} disabled={!importFile || importLoading} class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">
					{importLoading ? m.page_profile_importing() : m.page_profile_btn_import()}
				</button>
			</div>
			{#if importError}
				<div class="bg-red-50 border border-red-200 rounded p-2 text-red-800 text-sm">{importError}</div>
			{/if}
			{#if importResult}
				<div class="bg-green-50 border border-green-200 rounded p-3 text-sm space-y-1">
					<p class="text-green-800 font-medium">{m.page_profile_import_result({ count: String(importResult.imported) })}</p>
					{#if importResult.skipped > 0}
						<p class="text-orange-700">{m.page_profile_import_skipped({ skipped: String(importResult.skipped) })}</p>
					{/if}
					{#if importResult.errors?.length > 0}
						<details class="text-red-700">
							<summary class="cursor-pointer">{m.page_profile_import_errors({ errors: String(importResult.errors.length) })}</summary>
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
			<h3 class="font-medium mb-2">{m.page_profile_language_label()}</h3>
			<div class="flex items-center gap-3">
				<label class="text-sm text-neutral-700" for="group-language">{m.page_profile_group_language_label()}</label>
				<select id="group-language" bind:value={groupLanguage} class="border rounded px-2 py-1 text-sm">
					<option value="sv">Svenska</option>
					<option value="en">English</option>
				</select>
				<button onclick={saveGroupLanguage} class="text-sm bg-blue-700 text-white px-3 py-1 rounded">{m.btn_save()}</button>
				{#if groupLanguageMessage}<span class="text-sm text-green-600">{groupLanguageMessage}</span>{/if}
			</div>
		</section>

		<!-- SMTP settings -->
		<section class="mb-6 border rounded-lg p-4">
			<h3 class="font-medium mb-2">{m.page_profile_notifications_heading()}</h3>
			{#if settingsMessage}
				<div class="bg-green-50 border border-green-200 rounded p-2 mb-2 text-green-800 text-sm">{settingsMessage}</div>
			{/if}
			{#if settingsError}
				<div class="bg-red-50 border border-red-200 rounded p-2 mb-2 text-red-800 text-sm">{settingsError}</div>
			{/if}
				<div class="space-y-3">
					<label class="flex items-center gap-2 text-sm cursor-pointer">
						<input type="checkbox" bind:checked={useGroupSmtp} class="h-4 w-4 accent-blue-700" />
						{m.page_profile_smtp_use_group()}
					</label>
					{#if !useGroupSmtp}
						{#if groupSettings?.system_smtp_from}
							<p class="text-sm text-neutral-600">
								{m.page_profile_smtp_system_sender()}: <span class="font-mono">{groupSettings.system_smtp_from}</span>
							</p>
						{:else}
							<p class="text-sm text-amber-700">{m.page_profile_smtp_system_not_configured()}</p>
						{/if}
					{/if}
					{#if useGroupSmtp}
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
						<label class="block sm:col-span-2">
							<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_email_from()}</span>
							<input type="email" bind:value={settingsForm.notification_email_from} placeholder="utrustning@example.com" class="border rounded px-2 py-1 text-sm w-full max-w-sm" />
						</label>
						<label class="block">
							<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_smtp_host()}</span>
							<input type="text" bind:value={settingsForm.smtp_host} placeholder="smtp.example.com" class="border rounded px-2 py-1 text-sm w-full" />
						</label>
						<label class="block">
							<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_smtp_user()}</span>
							<input type="text" bind:value={settingsForm.smtp_user} placeholder="apikey" class="border rounded px-2 py-1 text-sm w-full" />
						</label>
						<label class="block">
							<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_smtp_port()}</span>
							<input type="number" bind:value={settingsForm.smtp_port} min="1" max="65535" class="border rounded px-2 py-1 text-sm w-full" />
						</label>
						<label class="block">
							<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_smtp_tls()}</span>
							<select bind:value={settingsForm.smtp_tls} class="border rounded px-2 py-1 text-sm w-full">
								<option value="starttls">{m.page_profile_smtp_tls_starttls()}</option>
								<option value="tls">{m.page_profile_smtp_tls_tls()}</option>
							</select>
						</label>
					</div>
					<label class="block">
						<span class="text-sm text-neutral-600 block mb-1">
							{m.page_profile_smtp_key()}
							{#if groupSettings?.smtp_key_set}
								<span class="text-neutral-400 ml-1">({groupSettings.smtp_key_masked})</span>
							{/if}
						</span>
						<input type="password" bind:value={settingsForm.smtp_key} placeholder={groupSettings?.smtp_key_set ? m.page_profile_leave_blank() : m.page_profile_smtp_enter()} class="border rounded px-2 py-1 text-sm w-full max-w-sm" />
					</label>
					{/if}
				<button onclick={saveSettings} class="text-sm bg-blue-700 text-white px-4 py-2 rounded">{m.btn_save()}</button>
				<div class="pt-2 border-t flex flex-wrap items-center gap-3">
					<button onclick={sendTestEmail} disabled={testEmailSending} class="text-sm text-blue-700 border border-blue-200 bg-blue-50 rounded px-3 py-1 hover:bg-blue-100 disabled:opacity-50">
						{testEmailSending ? m.page_profile_notifs_send_test_sending() : m.page_profile_notifs_send_test()}
					</button>
					<span class="text-xs text-neutral-400">{user.email}</span>
					{#if testEmailMessage}
						<span class="text-sm {testEmailError ? 'text-red-600' : 'text-green-600'}">{testEmailMessage}</span>
					{/if}
				</div>
			</div>
		</section>

			<!-- Google Chat integration -->
			<section class="mb-6 border rounded-lg p-4">
				<h3 class="font-medium mb-3">{m.page_profile_gchat_heading()}</h3>
				{#if groupSettings?.gchat_configured}
					<div class="flex items-center gap-3 mb-4">
						<span class="text-sm text-green-700 font-medium">{m.page_profile_gchat_connected_status()}</span>
						<span class="text-sm text-neutral-500">{m.page_profile_gchat_admin_email_label()}: {groupSettings.gchat_admin_email}</span>
						<button
							onclick={disconnectGChat}
							class="ml-auto text-sm text-red-600 border border-red-200 rounded px-3 py-1 hover:bg-red-50"
						>{m.page_profile_gchat_disconnect()}</button>
					</div>

					<!-- Team-space mapper (collapsible) -->
					<div class="border rounded">
						<button
							onclick={() => { gchatTeamMapperOpen = !gchatTeamMapperOpen; }}
							class="w-full flex items-center justify-between px-3 py-2 text-sm font-medium text-neutral-700 hover:bg-neutral-50"
						>
							{m.page_profile_gchat_team_mapper_heading()}
							<span class="text-neutral-400">{gchatTeamMapperOpen ? '▲' : '▼'}</span>
						</button>
						{#if gchatTeamMapperOpen}
							<div class="border-t p-3">
								{#if gchatTeams.length === 0}
									<p class="text-sm text-neutral-400">{m.btn_loading()}</p>
								{:else}
									<table class="w-full text-sm">
										<thead>
											<tr>
												<th class="text-left py-1 pr-3 font-normal text-neutral-500">{m.page_profile_gchat_team_column()}</th>
												<th class="text-left py-1 font-normal text-neutral-500">{m.page_profile_gchat_space_column()}</th>
											</tr>
										</thead>
										<tbody>
											{#each gchatTeams as gt}
											<tr class="border-t border-neutral-100">
												<td class="py-1.5 pr-3">{gt.name}</td>
												<td class="py-1.5">
													<select
														value={gt.gchat_space_id}
														onchange={(e) => setGchatTeamSpace(gt.id, e.currentTarget.value)}
														disabled={gchatTeamSaving[gt.id]}
														class="border rounded px-2 py-1 text-sm w-full max-w-xs disabled:opacity-50"
													>
														<option value="">{m.page_profile_gchat_space_none_option()}</option>
														{#each gchatSpaces as space}
															<option value={space.name}>{space.displayName} ({space.name})</option>
														{/each}
													</select>
												</td>
											</tr>
											{/each}
										</tbody>
									</table>
								{/if}
							</div>
						{/if}
					</div>
				{:else}
					<p class="text-sm text-neutral-500 mb-3">
						{m.page_profile_gchat_paste_hint()}
						<a href="/docs/gchat" target="_blank" class="text-blue-700 underline ml-1">{m.page_profile_gchat_setup_guide()}</a>
					</p>
					<label class="block mb-3" for="gchat-key-json">
						<span class="text-sm text-neutral-600 block mb-1">{m.page_profile_gchat_paste_label()}</span>
						<textarea
							id="gchat-key-json"
							bind:value={gchatKeyJson}
							rows={3}
							class="border rounded px-2 py-1 text-sm w-full font-mono"
							placeholder='{"{"}  "type": "service_account", ...'
						></textarea>
					</label>
					{#if gchatError}
						<p class="text-sm text-red-600 mb-2">{gchatError}</p>
					{/if}
					<button
						onclick={connectGChat}
						disabled={gchatConnecting || !gchatKeyJson.trim()}
						class="text-sm bg-blue-700 text-white px-4 py-2 rounded disabled:opacity-50"
					>{gchatConnecting ? m.page_profile_gchat_connecting() : m.page_profile_gchat_connect()}</button>
				{/if}
			</section>

			<!-- Group notification defaults -->
			<section class="mb-6 border rounded-lg p-4">
				<h3 class="font-medium mb-1">{m.page_profile_notif_defaults_heading()}</h3>
				<p class="text-xs text-neutral-500 mb-3">{m.page_profile_notif_defaults_help()}</p>
				{#if groupNotifDefaultsMessage}
					<p class="text-sm mb-2 {groupNotifDefaultsError ? 'text-red-600' : 'text-green-600'}">{groupNotifDefaultsMessage}</p>
				{/if}

				<!-- Default Gruppkanal composition -->
				<div class="mb-5">
					<h4 class="text-sm font-medium text-neutral-700 mb-1">{m.page_profile_group_defaults_gruppkanal_heading()}</h4>
					<p class="text-xs text-neutral-400 mb-2">{m.page_profile_group_defaults_gruppkanal_help()}</p>
					{#each notifChannels as ch}
						<label class="flex items-center gap-2 mb-1 cursor-pointer">
							<input
								type="checkbox"
								checked={groupDefaultGruppkanalChannels.includes(ch)}
								onchange={(e) => {
									const updated = e.currentTarget.checked
										? [...groupDefaultGruppkanalChannels, ch]
										: groupDefaultGruppkanalChannels.filter(c => c !== ch);
									saveGroupDefaults({ default_gruppkanal_channels: updated });
								}}
								class="h-4 w-4 accent-blue-700"
							/>
							<span class="text-sm text-neutral-700">
								{ch === 'email' ? m.page_profile_teams_gruppkanal_channel_email() : m.page_profile_teams_gruppkanal_channel_gchat()}
							</span>
						</label>
					{/each}
				</div>

				<!-- Per-event defaults table -->
				<div class="overflow-x-auto">
				<table class="w-full text-sm border-collapse">
					<thead>
						<tr>
							<th class="text-left py-1 pr-3 font-normal text-neutral-500 w-full"></th>
							<th class="text-center py-1 px-2 font-normal text-neutral-500 whitespace-nowrap">{m.page_profile_teams_notif_col_personal()}</th>
							<th class="text-center py-1 px-3 font-normal text-neutral-500 whitespace-nowrap">{m.page_profile_teams_notif_col_gruppkanal()}</th>
						</tr>
					</thead>
					<tbody>
						<tr>
							<td colspan="3" class="py-1.5 text-xs font-semibold text-neutral-500 uppercase tracking-wide">{m.page_profile_notifs_bookings_group()}</td>
						</tr>
						{#each allBookingEvents.filter(r => !managerOnlyKeys.has(r.key)) as row}
						{@const ep = groupNotifDefaults[row.key]}
						{@const sysEp = groupSysDefaults[row.key]}
						{@const policy = ep?.personal_email_policy ?? ''}
						{@const grEnabled = ep?.gruppkanal}
						<tr class="border-t border-neutral-100">
							<td class="py-1.5 pr-3">{row.label()}</td>
							<td class="py-1.5 px-2">
								<div class="flex gap-1 justify-center">
									{#each [['always', m.page_profile_teams_notif_personal_always()], ['if_no_broadcast', m.page_profile_teams_notif_personal_if_no_broadcast()], ['never', m.page_profile_teams_notif_personal_never()]] as [val, label]}
										<button
											onclick={() => saveGroupDefaults({ defaults: { ...groupNotifDefaults, [row.key]: { ...(groupNotifDefaults[row.key] ?? {}), personal_email_policy: val as 'always' | 'if_no_broadcast' | 'never' } } })}
											class="px-2 py-0.5 text-xs rounded border {(policy || sysEp?.personal_email_policy || 'if_no_broadcast') === val ? 'bg-blue-700 text-white border-blue-700' : 'bg-white text-neutral-600 border-neutral-300 hover:border-blue-400'}"
										>{label}</button>
									{/each}
								</div>
							</td>
							<td class="py-1.5 px-3 text-center">
								<input
									type="checkbox"
									checked={grEnabled ?? sysEp?.gruppkanal ?? true}
									onchange={(e) => saveGroupDefaults({ defaults: { ...groupNotifDefaults, [row.key]: { ...(groupNotifDefaults[row.key] ?? {}), gruppkanal: e.currentTarget.checked } } })}
									class="h-4 w-4 accent-blue-700"
								/>
							</td>
						</tr>
						{/each}
						<tr>
							<td colspan="3" class="pt-3 pb-1.5 text-xs font-semibold text-neutral-500 uppercase tracking-wide">{m.page_profile_notif_defaults_section_manager()}</td>
						</tr>
						{#each [...allBookingEvents.filter(r => managerOnlyKeys.has(r.key)), ...allIssueEvents] as row}
						{@const ep = groupNotifDefaults[row.key]}
						{@const sysEp = groupSysDefaults[row.key]}
						{@const policy = ep?.personal_email_policy ?? ''}
						{@const grEnabled = ep?.gruppkanal}
						<tr class="border-t border-neutral-100">
							<td class="py-1.5 pr-3">{row.label()}</td>
							<td class="py-1.5 px-2">
								<div class="flex gap-1 justify-center">
									{#each [['always', m.page_profile_teams_notif_personal_always()], ['if_no_broadcast', m.page_profile_teams_notif_personal_if_no_broadcast()], ['never', m.page_profile_teams_notif_personal_never()]] as [val, label]}
										<button
											onclick={() => saveGroupDefaults({ defaults: { ...groupNotifDefaults, [row.key]: { ...(groupNotifDefaults[row.key] ?? {}), personal_email_policy: val as 'always' | 'if_no_broadcast' | 'never' } } })}
											class="px-2 py-0.5 text-xs rounded border {(policy || sysEp?.personal_email_policy || 'if_no_broadcast') === val ? 'bg-blue-700 text-white border-blue-700' : 'bg-white text-neutral-600 border-neutral-300 hover:border-blue-400'}"
										>{label}</button>
									{/each}
								</div>
							</td>
							<td class="py-1.5 px-3 text-center">
								<input
									type="checkbox"
									checked={grEnabled ?? sysEp?.gruppkanal ?? true}
									onchange={(e) => saveGroupDefaults({ defaults: { ...groupNotifDefaults, [row.key]: { ...(groupNotifDefaults[row.key] ?? {}), gruppkanal: e.currentTarget.checked } } })}
									class="h-4 w-4 accent-blue-700"
								/>
							</td>
						</tr>
						{/each}
					</tbody>
				</table>
				</div>
				<div class="mt-4 pt-3 border-t flex flex-wrap items-center gap-3">
					<button
						onclick={forceNotificationDefaults}
						disabled={forceDefaultsRunning}
						class="text-sm text-orange-700 border border-orange-200 bg-orange-50 rounded px-3 py-1.5 hover:bg-orange-100 disabled:opacity-50"
					>{forceDefaultsRunning ? m.btn_saving() : m.page_profile_notif_force_defaults_btn()}</button>
					<span class="text-xs text-neutral-400">{m.page_profile_notif_force_defaults_help()}</span>
				</div>
			</section>
	{/if}
</div>
{/if}
