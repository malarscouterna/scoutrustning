<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';
	import ImageViewer from '$lib/components/ImageViewer.svelte';
	import ReportIssueSheet from '$lib/components/ReportIssueSheet.svelte';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		bookingId: string;
		items: BookingItem[];
		onUpdate: () => Promise<BookingItem[]>;
		onBookingReturned: () => void;
	}

	let { bookingId, items, onUpdate, onBookingReturned }: Props = $props();
	const api = createApiClient();

	let error = $state('');
	let savedKey = $state<string | null>(null);
	let activeItemId = $state<string | null>(null);
	let activeGroupKey = $state<string | null>(null);
	let form = $state({ status: '', expectedReturnDate: '', notes: '' });
	let lastExpectedDate = $state('');
	let delayWarning = $state('');
	let quantityInputs = $state<Record<string, number>>({});
	let expandedGroups = $state<Set<string>>(new Set());
	let completing = $state(false);

	const labels = $derived<Record<string, string>>({
		returned_ok: m.return_status_ok(),
		delayed: m.return_status_late(),
		reported_usable: m.return_status_problem_usable(),
		reported_unusable: m.return_status_problem_unusable(),
		missing: m.return_status_missing()
	});
	const colors: Record<string, string> = { returned_ok: 'bg-green-100 text-green-800', delayed: 'bg-orange-100 text-orange-800', reported_usable: 'bg-orange-100 text-orange-800', reported_unusable: 'bg-red-100 text-red-800', missing: 'bg-challengerpink-100 text-challengerpink-800' };

	const returnStatusToSeverity: Record<string, string> = {
		reported_usable: 'usable',
		reported_unusable: 'unusable',
		missing: 'missing'
	};

	let issueSheetArticle = $state<{ id: string; name: string; severity: string } | null>(null);

	interface QGroup { key: string; name: string; loc: string; place: string; picked: BookingItem[]; notPicked: number; }
	interface QRow { status: string; count: number; items: BookingItem[]; }

	let tracked = $derived(items.filter((i) => i.individually_tracked));
	let groups = $derived.by(() => {
		const m = new Map<string, QGroup>();
		for (const i of items) {
			if (i.individually_tracked) continue;
			const k = `${i.commercial_name}|${i.location_name}`;
			const isPicked = i.pickup_status;
			const g = m.get(k);
			if (g) { if (isPicked) g.picked.push(i); else g.notPicked++; }
			else m.set(k, { key: k, name: i.commercial_name, loc: i.location_name, place: i.place, picked: isPicked ? [i] : [], notPicked: isPicked ? 0 : 1 });
		}
		return [...m.values()];
	});

	interface TrackedImageGroup {
		commercialName: string;
		imageIds: string[];
		locationId: string;
		description: string;
		instructions: string;
		items: BookingItem[];
	}
	let trackedImageGroups = $derived.by(() => {
		const map = new Map<string, TrackedImageGroup>();
		for (const item of tracked) {
			const existing = map.get(item.commercial_name);
			if (existing) {
				existing.items.push(item);
			} else {
				map.set(item.commercial_name, {
					commercialName: item.commercial_name,
					imageIds: item.image_ids ?? [],
					locationId: item.location_id,
					description: item.article_description ?? '',
					instructions: item.article_instructions ?? '',
					items: [item]
				});
			}
		}
		return [...map.values()];
	});

	// Clear stale active form when items update externally
	$effect(() => {
		if (activeItemId && !items.find(i => i.id === activeItemId && (!i.return_status || i.return_status === 'pending'))) {
			activeItemId = null;
		}
		if (activeGroupKey) {
			const g = groups.find(g => g.key === activeGroupKey);
			if (!g || g.picked.filter(i => !i.return_status || i.return_status === 'pending').length === 0) {
				activeGroupKey = null;
			}
		}
	});

	function toggleExpand(key: string) {
		const next = new Set(expandedGroups);
		if (next.has(key)) next.delete(key); else next.add(key);
		expandedGroups = next;
	}

	function hasExpandable(imageIds: string[], desc: string, instr: string): boolean {
		return imageIds.length > 0 || !!desc || !!instr;
	}

	function groupRows(g: QGroup): QRow[] {
		const byStatus = new Map<string, BookingItem[]>();
		for (const i of g.picked) {
			const s = (i.return_status && i.return_status !== 'pending') ? i.return_status : '_unhandled';
			byStatus.set(s, [...(byStatus.get(s) ?? []), i]);
		}
		const rows: QRow[] = [];
		for (const [s, items] of byStatus) {
			if (s !== '_unhandled') rows.push({ status: s, count: items.length, items });
		}
		const unhandled = byStatus.get('_unhandled');
		if (unhandled) rows.push({ status: '_unhandled', count: unhandled.length, items: unhandled });
		return rows;
	}

	let pickedUp = $derived(items.filter((i) => i.pickup_status));
	let returnedCount = $derived(pickedUp.filter((i) => i.return_status && i.return_status !== 'pending').length);
	let canComplete = $derived(pickedUp.length > 0 && pickedUp.every((i) => i.return_status && i.return_status !== 'pending' && i.return_status !== 'delayed'));

	function flash(key: string) { savedKey = key; setTimeout(() => { if (savedKey === key) savedKey = null; }, 2000); }

	async function setReturn(itemId: string, status: string, extra?: { expected_return_date?: string; notes?: string; image_ids?: string[] }) {
		error = '';
		try {
			await api.updateItemReturn(bookingId, itemId, { return_status: status, ...extra });
			await onUpdate(); flash(itemId);
		} catch (e: any) { error = e.message; }
	}

	async function confirmForm(item: BookingItem) {
		if (!form.status) return;
		if (form.status === 'delayed' && form.expectedReturnDate) lastExpectedDate = form.expectedReturnDate;
		const severity = returnStatusToSeverity[form.status];
		await setReturn(item.id, form.status, {
			expected_return_date: form.status === 'delayed' ? form.expectedReturnDate : undefined,
			notes: severity ? undefined : form.notes || undefined,
		});
		activeItemId = null;
		if (severity) {
			issueSheetArticle = { id: item.article_id, name: item.common_name, severity };
		}
	}

	function openForm(id: string) {
		activeItemId = id; activeGroupKey = null;
		form = { status: '', expectedReturnDate: lastExpectedDate, notes: '' }; delayWarning = '';
	}

	async function returnGroupOk(g: QGroup) {
		error = '';
		const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending');
		const count = Math.min(quantityInputs[g.key] ?? unhandled.length, unhandled.length);
		try {
			for (let i = 0; i < count; i++)
				await api.updateItemReturn(bookingId, unhandled[i].id, { return_status: 'returned_ok' });
			delete quantityInputs[g.key];
			await onUpdate(); flash(g.key);
		} catch (e: any) { error = e.message; }
	}

	async function confirmGroupForm(g: QGroup) {
		if (!form.status) return;
		error = '';
		const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending');
		const count = Math.min(quantityInputs[`${g.key}_form`] ?? 1, unhandled.length);
		if (form.status === 'delayed' && form.expectedReturnDate) lastExpectedDate = form.expectedReturnDate;
		const severity = returnStatusToSeverity[form.status];
		try {
			for (let i = 0; i < count; i++)
				await api.updateItemReturn(bookingId, unhandled[i].id, {
					return_status: form.status,
					expected_return_date: form.status === 'delayed' ? form.expectedReturnDate : undefined,
					notes: severity ? undefined : form.notes || undefined,
				});
			activeGroupKey = null;
			delete quantityInputs[`${g.key}_form`];
			await onUpdate(); flash(g.key);
			if (severity && unhandled[0]) {
				issueSheetArticle = { id: unhandled[0].article_id, name: g.name, severity };
			}
		} catch (e: any) { error = e.message; }
	}

	async function undoGroupRow(row: QRow) {
		error = '';
		try {
			for (const i of row.items) await api.updateItemReturn(bookingId, i.id, { return_status: '' });
			await onUpdate();
		} catch (e: any) { error = e.message; }
	}

	function openGroupForm(g: QGroup) {
		const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending');
		activeGroupKey = g.key; activeItemId = null;
		form = { status: '', expectedReturnDate: lastExpectedDate, notes: '' }; delayWarning = '';
		// Always reset to current unhandled count (Issue 11)
		quantityInputs[`${g.key}_form`] = unhandled.length;
	}

	async function checkConflict(name: string, date: string) {
		delayWarning = '';
		if (!date) return;
		try {
			const a = await api.checkAvailability(date, date);
			const g = a.find((x) => x.commercial_name === name);
			if (!g || g.available_count === 0) delayWarning = `${name} är fullbokad ${date}`;
		} catch {}
	}
</script>

{#snippet infoBlock(imageIds: string[], commercialName: string, locationId: string, description: string, instructions: string)}
	<div class="px-4 py-2 border-t space-y-2 text-xs text-neutral-600">
		{#if imageIds.length > 0}
			<ImageViewer {imageIds} alt={commercialName} {commercialName} {locationId} />
		{/if}
		{#if description}
			<div>
				<span class="font-medium text-neutral-500">{m.lbl_description_colon()}</span>
				<p class="mt-0.5">{description}</p>
			</div>
		{/if}
		{#if instructions}
			<div>
				<span class="font-medium text-neutral-500">{m.lbl_instructions_colon()}</span>
				<p class="mt-0.5">{instructions}</p>
			</div>
		{/if}
	</div>
{/snippet}

{#if error}<div class="bg-red-50 border border-red-200 rounded p-3 mb-3 text-red-800 text-sm">{error}</div>{/if}
<p class="text-sm text-neutral-500 mb-3">{m.return_returned_label()} {returnedCount} / {pickedUp.length}</p>

<div class="space-y-1">
	{#each groups as g}
		{@const rep = g.picked[0] ?? items.find(i => !i.individually_tracked && i.commercial_name === g.name)}
		{@const gImageIds = rep?.image_ids ?? []}
		{@const expandable = hasExpandable(gImageIds, rep?.article_description ?? '', rep?.article_instructions ?? '')}
		{@const expanded = expandedGroups.has(g.key)}
		{#if g.picked.length === 0}
			<div class="border rounded">
				<div class="px-4 py-3 flex items-center gap-3 bg-neutral-50">
					<button type="button" onclick={() => expandable && toggleExpand(g.key)} class="flex-1 text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
						<div class="font-medium text-sm">{g.name}{#if expandable}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}</div>
						<div class="text-xs text-neutral-500">{g.loc}{g.place ? ` · ${g.place}` : ''}</div>
					</button>
					<span class="text-xs text-neutral-400">{m.return_not_picked_up()} ({g.notPicked} st)</span>
				</div>
				{#if expanded}
					{@render infoBlock(gImageIds, g.name, rep?.location_id ?? '', rep?.article_description ?? '', rep?.article_instructions ?? '')}
				{/if}
			</div>
		{:else}
			{@const rows = groupRows(g)}
			{#each rows as row}
				<div class="border rounded"
					class:bg-green-50={row.status === 'returned_ok'}
					class:bg-orange-50={row.status === 'delayed' || row.status === 'reported_usable'}
					class:bg-red-50={row.status === 'reported_unusable'}
					class:bg-challengerpink-50={row.status === 'missing'}
				>
					<div class="px-4 py-3 flex items-center gap-3">
						<button type="button" onclick={() => expandable && toggleExpand(g.key)} class="flex-1 text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
							<div class="font-medium text-sm">{g.name} × {row.count}{#if expandable && rows[0] === row}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}</div>
							<div class="text-xs text-neutral-500">{g.loc}{g.place ? ` · ${g.place}` : ''}{#if g.notPicked > 0}<span class="text-orange-600"> · {g.notPicked} {m.return_not_picked_up()}</span>{/if}</div>
						</button>

						{#if row.status === '_unhandled' && activeGroupKey !== g.key}
							{@const unhandledCount = row.count}
							<input type="number" min="0" max={unhandledCount} value={quantityInputs[g.key] ?? unhandledCount} oninput={(e) => quantityInputs[g.key] = parseInt(e.currentTarget.value) || 0} class="w-16 text-center border rounded px-2 py-1 text-sm" />
							<button onclick={() => returnGroupOk(g)} class="text-xs bg-green-700 text-white px-2 py-1 rounded">{m.btn_ok()}</button>
							<button onclick={() => openGroupForm(g)} class="text-xs border px-2 py-1 rounded text-neutral-700">{m.return_btn_other()}</button>
						{:else if row.status === '_unhandled'}
							<!-- hidden while form is open -->
						{:else}
							<span class="text-xs px-2 py-0.5 rounded {colors[row.status] ?? ''}">{labels[row.status] ?? row.status}</span>
							{#if savedKey === g.key}<span class="text-xs text-green-600">{m.return_saved()}</span>{/if}
							<button onclick={() => undoGroupRow(row)} class="text-xs text-neutral-400 hover:text-neutral-600">{m.btn_undo()}</button>
						{/if}
					</div>
				</div>
			{/each}

			{#if activeGroupKey === g.key}
				{@const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending')}
				<div class="border rounded p-3 bg-neutral-50 text-sm space-y-2">
					<div class="flex items-center gap-2">
						<span>{m.return_count_label()}</span>
						<input type="number" min="1" max={unhandled.length} value={quantityInputs[`${g.key}_form`] ?? 1} oninput={(e) => quantityInputs[`${g.key}_form`] = parseInt(e.currentTarget.value) || 1} class="w-16 text-center border rounded px-2 py-1" />
						<span class="text-neutral-500">{m.common_of()} {unhandled.length} {m.common_remaining()}</span>
					</div>
					<div class="flex flex-wrap gap-2">
						{#each ['returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'missing'] as s}
							<button onclick={() => form.status = s} class="text-xs px-3 py-1 rounded border" class:bg-blue-700={form.status === s} class:text-white={form.status === s}>{labels[s]}</button>
						{/each}
					</div>
					{#if form.status === 'delayed'}
						<label class="block"><span class="text-xs text-neutral-600">{m.return_expected_date()}</span>
							<input type="date" bind:value={form.expectedReturnDate} oninput={() => checkConflict(g.name, form.expectedReturnDate)} class="block border rounded px-2 py-1 text-sm w-full" /></label>
						{#if delayWarning}<p class="text-xs text-orange-600">⚠ {delayWarning}</p>{/if}
					{/if}
					{#if form.status === 'reported_usable' || form.status === 'reported_unusable' || form.status === 'missing'}
						<p class="text-xs text-neutral-500">{m.return_desc_hint()}</p>
					{/if}
					<div class="flex gap-2">
						<button onclick={() => confirmGroupForm(g)} disabled={!form.status || (form.status === 'delayed' && !form.expectedReturnDate)} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">{m.btn_confirm()}</button>
						<button onclick={() => activeGroupKey = null} class="text-xs text-neutral-600 underline">{m.btn_cancel()}</button>
					</div>
				</div>
			{/if}
			{#if expanded}
				<div class="border rounded">
					{@render infoBlock(gImageIds, g.name, rep?.location_id ?? '', rep?.article_description ?? '', rep?.article_instructions ?? '')}
				</div>
			{/if}
		{/if}
	{/each}

	{#each trackedImageGroups as tGroup}
		{@const tKey = tGroup.commercialName}
		{@const expandable = hasExpandable(tGroup.imageIds, tGroup.description, tGroup.instructions)}
		{@const expanded = expandedGroups.has(tKey)}
		{#each tGroup.items as item}
		{@const notPicked = !item.pickup_status}
		{@const hasReturn = item.return_status && item.return_status !== 'pending'}
		<div class="border rounded" class:bg-green-50={item.return_status === 'returned_ok'} class:bg-orange-50={item.return_status === 'delayed' || item.return_status === 'reported_usable'} class:bg-red-50={item.return_status === 'reported_unusable'} class:bg-challengerpink-50={item.return_status === 'missing'} class:bg-neutral-50={notPicked}>
			<div class="px-4 py-3 flex items-center gap-3">
				<button type="button" onclick={() => expandable && toggleExpand(tKey)} class="flex-1 text-left" class:cursor-pointer={expandable} class:cursor-default={!expandable}>
					<div class="font-medium text-sm">{item.common_name}{#if expandable && tGroup.items[0] === item}<span class="text-xs text-neutral-400 ml-1">{expanded ? '▲' : '▼'}</span>{/if}</div>
					<div class="text-xs text-neutral-500">{item.location_name}{item.place ? ` · ${item.place}` : ''}</div>
				</button>
				{#if notPicked}
					<span class="text-xs text-neutral-400">{m.return_not_picked_up()}</span>
				{:else if hasReturn}
					<span class="text-xs px-2 py-0.5 rounded {colors[item.return_status ?? ''] ?? ''}">{labels[item.return_status ?? ''] ?? item.return_status}</span>
					{#if savedKey === item.id}<span class="text-xs text-green-600">{m.return_saved()}</span>{/if}
					<button onclick={() => setReturn(item.id, '')} class="text-xs text-neutral-400 hover:text-neutral-600">{m.btn_undo()}</button>
				{:else if activeItemId !== item.id}
					<div class="flex gap-1">
						<button onclick={() => setReturn(item.id, 'returned_ok')} class="text-xs bg-green-700 text-white px-2 py-1 rounded">{m.btn_ok()}</button>
						<button onclick={() => openForm(item.id)} class="text-xs border px-2 py-1 rounded text-neutral-700">{m.return_btn_other()}</button>
					</div>
				{/if}
			</div>
			{#if expanded && tGroup.items[0] === item}
				{@render infoBlock(tGroup.imageIds, tGroup.commercialName, tGroup.locationId, tGroup.description, tGroup.instructions)}
			{/if}
		</div>
		{#if activeItemId === item.id}
			<div class="border rounded p-3 bg-neutral-50 text-sm space-y-2">
				<div class="flex flex-wrap gap-2">
					{#each ['returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'missing'] as s}
						<button onclick={() => form.status = s} class="text-xs px-3 py-1 rounded border" class:bg-blue-700={form.status === s} class:text-white={form.status === s}>{labels[s]}</button>
					{/each}
				</div>
				{#if form.status === 'delayed'}
					<label class="block"><span class="text-xs text-neutral-600">{m.return_expected_date()}</span>
						<input type="date" bind:value={form.expectedReturnDate} oninput={() => checkConflict(item.commercial_name, form.expectedReturnDate)} class="block border rounded px-2 py-1 text-sm w-full" /></label>
					{#if delayWarning}<p class="text-xs text-orange-600">⚠ {delayWarning}</p>{/if}
				{/if}
				{#if form.status === 'reported_usable' || form.status === 'reported_unusable' || form.status === 'missing'}
					<p class="text-xs text-neutral-500">{m.return_desc_hint()}</p>
				{/if}
				<div class="flex gap-2">
					<button onclick={() => confirmForm(item)} disabled={!form.status || (form.status === 'delayed' && !form.expectedReturnDate)} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">{m.btn_confirm()}</button>
					<button onclick={() => activeItemId = null} class="text-xs text-neutral-600 underline">{m.btn_cancel()}</button>
				</div>
			</div>
		{/if}
		{/each}
	{/each}
</div>

{#if canComplete}
	<div class="mt-4">
		<button
			disabled={completing}
			onclick={async () => {
				completing = true;
				try {
					await api.returnBooking(bookingId);
					onBookingReturned();
				} catch (e: any) {
					error = e.message;
				} finally {
					completing = false;
				}
			}}
			class="bg-green-700 text-white px-4 py-2 rounded text-sm disabled:opacity-50"
		>{m.return_btn_finish()}</button>
	</div>
{/if}

{#if issueSheetArticle}
	<ReportIssueSheet
		articleId={issueSheetArticle.id}
		articleName={issueSheetArticle.name}
		open={true}
		severity={issueSheetArticle.severity}
		bookingId={bookingId}
		onClose={() => issueSheetArticle = null}
	/>
{/if}
