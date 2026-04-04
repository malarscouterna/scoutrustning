<script lang="ts">
	import { createApiClient, type BookingItem } from '$lib/api/client';

	interface Props {
		bookingId: string;
		items: BookingItem[];
		onUpdate: (items: BookingItem[]) => void;
		onBookingReturned: () => void;
	}

	let { bookingId, items, onUpdate, onBookingReturned }: Props = $props();
	const api = createApiClient({ persona: 'leader-yggdrasil' });

	let error = $state('');
	let savedKey = $state<string | null>(null);
	let activeItemId = $state<string | null>(null);
	let activeGroupKey = $state<string | null>(null);
	let form = $state({ status: '', expectedReturnDate: '', notes: '' });
	let lastExpectedDate = $state('');
	let delayWarning = $state('');
	let quantityInputs = $state<Record<string, number>>({});

	const labels: Record<string, string> = { returned_ok: 'OK', delayed: 'Försenad', reported_usable: 'Problem — användbar', reported_unusable: 'Problem — ej användbar', lost: 'Saknas' };
	const colors: Record<string, string> = { returned_ok: 'bg-green-100 text-green-800', delayed: 'bg-orange-100 text-orange-800', reported_usable: 'bg-orange-100 text-orange-800', reported_unusable: 'bg-red-100 text-red-800', lost: 'bg-challengerpink-100 text-challengerpink-800' };

	interface QGroup { key: string; name: string; loc: string; place: string; picked: BookingItem[]; notPicked: number; }
	interface QRow { status: string; count: number; items: BookingItem[]; }

	let tracked = $derived(items.filter((i) => i.individually_tracked));
	let groups = $derived.by(() => {
		const m = new Map<string, QGroup>();
		for (const i of items) {
			if (i.individually_tracked) continue;
			const k = `${i.commercial_name}|${i.location_name}`;
			const isPicked = i.pickup_status && i.pickup_status !== 'lost';
			const g = m.get(k);
			if (g) { if (isPicked) g.picked.push(i); else g.notPicked++; }
			else m.set(k, { key: k, name: i.commercial_name, loc: i.location_name, place: i.place, picked: isPicked ? [i] : [], notPicked: isPicked ? 0 : 1 });
		}
		return [...m.values()];
	});

	function groupRows(g: QGroup): QRow[] {
		const byStatus = new Map<string, BookingItem[]>();
		for (const i of g.picked) {
			const s = (i.return_status && i.return_status !== 'pending') ? i.return_status : '_unhandled';
			byStatus.set(s, [...(byStatus.get(s) ?? []), i]);
		}
		const rows: QRow[] = [];
		// Show handled statuses first, unhandled last
		for (const [s, items] of byStatus) {
			if (s !== '_unhandled') rows.push({ status: s, count: items.length, items });
		}
		const unhandled = byStatus.get('_unhandled');
		if (unhandled) rows.push({ status: '_unhandled', count: unhandled.length, items: unhandled });
		return rows;
	}

	let pickedUp = $derived(items.filter((i) => i.pickup_status && i.pickup_status !== 'lost'));
	let returnedCount = $derived(pickedUp.filter((i) => i.return_status && i.return_status !== 'pending').length);
	let canComplete = $derived(pickedUp.length > 0 && pickedUp.every((i) => i.return_status && i.return_status !== 'pending' && i.return_status !== 'delayed'));

	async function reload() { onUpdate((await api.getBooking(bookingId)).items); }
	function flash(key: string) { savedKey = key; setTimeout(() => { if (savedKey === key) savedKey = null; }, 2000); }

	// --- Individual items ---
	async function setReturn(itemId: string, status: string, extra?: { expected_return_date?: string; notes?: string }) {
		error = '';
		try {
			await api.updateItemReturn(bookingId, itemId, { return_status: status, ...extra });
			await reload(); flash(itemId);
		} catch (e: any) { error = e.message; }
	}

	async function confirmForm(itemId: string) {
		if (!form.status) return;
		if (form.status === 'delayed' && form.expectedReturnDate) lastExpectedDate = form.expectedReturnDate;
		await setReturn(itemId, form.status, {
			expected_return_date: form.status === 'delayed' ? form.expectedReturnDate : undefined,
			notes: form.notes || undefined
		});
		activeItemId = null;
	}

	function openForm(id: string) {
		activeItemId = id; activeGroupKey = null;
		form = { status: '', expectedReturnDate: lastExpectedDate, notes: '' }; delayWarning = '';
	}

	// --- Quantity groups ---
	async function returnGroupOk(g: QGroup) {
		error = '';
		const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending');
		const count = Math.min(quantityInputs[g.key] ?? unhandled.length, unhandled.length);
		try {
			for (let i = 0; i < count; i++)
				await api.updateItemReturn(bookingId, unhandled[i].id, { return_status: 'returned_ok' });
			delete quantityInputs[g.key];
			await reload(); flash(g.key);
		} catch (e: any) { error = e.message; }
	}

	async function confirmGroupForm(g: QGroup) {
		if (!form.status) return;
		error = '';
		const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending');
		const count = Math.min(quantityInputs[`${g.key}_form`] ?? 1, unhandled.length);
		if (form.status === 'delayed' && form.expectedReturnDate) lastExpectedDate = form.expectedReturnDate;
		try {
			for (let i = 0; i < count; i++)
				await api.updateItemReturn(bookingId, unhandled[i].id, {
					return_status: form.status,
					expected_return_date: form.status === 'delayed' ? form.expectedReturnDate : undefined,
					notes: form.notes || undefined
				});
			activeGroupKey = null;
			delete quantityInputs[`${g.key}_form`];
			await reload(); flash(g.key);
		} catch (e: any) { error = e.message; }
	}

	async function undoGroupRow(row: QRow) {
		error = '';
		try {
			for (const i of row.items) await api.updateItemReturn(bookingId, i.id, { return_status: '' });
			await reload();
		} catch (e: any) { error = e.message; }
	}

	function openGroupForm(g: QGroup) {
		const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending');
		activeGroupKey = g.key; activeItemId = null;
		form = { status: '', expectedReturnDate: lastExpectedDate, notes: '' }; delayWarning = '';
		quantityInputs[`${g.key}_form`] = quantityInputs[g.key] ?? unhandled.length;
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

{#if error}<div class="bg-red-50 border border-red-200 rounded p-3 mb-3 text-red-800 text-sm">{error}</div>{/if}
<p class="text-sm text-neutral-500 mb-3">Återlämnad: {returnedCount} / {pickedUp.length}</p>

<div class="space-y-1">
	{#each groups as g}
		{#if g.picked.length === 0}
			<div class="border rounded px-4 py-3 flex items-center gap-3 bg-neutral-50">
				<div class="flex-1"><div class="font-medium text-sm">{g.name}</div><div class="text-xs text-neutral-500">{g.loc}{g.place ? ` · ${g.place}` : ''}</div></div>
				<span class="text-xs text-neutral-400">Ej hämtad ({g.notPicked} st)</span>
			</div>
		{:else}
			{@const rows = groupRows(g)}
			{#each rows as row}
				<div class="border rounded px-4 py-3 flex items-center gap-3"
					class:bg-green-50={row.status === 'returned_ok'}
					class:bg-orange-50={row.status === 'delayed' || row.status === 'reported_usable'}
					class:bg-red-50={row.status === 'reported_unusable'}
					class:bg-challengerpink-50={row.status === 'lost'}
				>
					<div class="flex-1">
						<div class="font-medium text-sm">{g.name} × {row.count}</div>
						<div class="text-xs text-neutral-500">{g.loc}{g.place ? ` · ${g.place}` : ''}{#if g.notPicked > 0}<span class="text-orange-600"> · {g.notPicked} ej hämtade</span>{/if}</div>
					</div>

					{#if row.status === '_unhandled' && activeGroupKey !== g.key}
						{@const unhandledCount = row.count}
						<input type="number" min="0" max={unhandledCount} value={quantityInputs[g.key] ?? unhandledCount} oninput={(e) => quantityInputs[g.key] = parseInt(e.currentTarget.value) || 0} class="w-16 text-center border rounded px-2 py-1 text-sm" />
						<button onclick={() => returnGroupOk(g)} class="text-xs bg-green-700 text-white px-2 py-1 rounded">OK</button>
						<button onclick={() => openGroupForm(g)} class="text-xs border px-2 py-1 rounded text-neutral-700">Annat...</button>
					{:else if row.status === '_unhandled'}
						<!-- hidden while form is open -->
					{:else}
						<span class="text-xs px-2 py-0.5 rounded {colors[row.status] ?? ''}">{labels[row.status] ?? row.status}</span>
						{#if savedKey === g.key}<span class="text-xs text-green-600">Sparad</span>{/if}
						<button onclick={() => undoGroupRow(row)} class="text-xs text-neutral-400 hover:text-neutral-600">Ångra</button>
					{/if}
				</div>
			{/each}

			{#if activeGroupKey === g.key}
				{@const unhandled = g.picked.filter((i) => !i.return_status || i.return_status === 'pending')}
				<div class="border rounded p-3 bg-neutral-50 text-sm space-y-2">
					<div class="flex items-center gap-2">
						<span>Antal:</span>
						<input type="number" min="1" max={unhandled.length} value={quantityInputs[`${g.key}_form`] ?? 1} oninput={(e) => quantityInputs[`${g.key}_form`] = parseInt(e.currentTarget.value) || 1} class="w-16 text-center border rounded px-2 py-1" />
						<span class="text-neutral-500">av {unhandled.length} kvar</span>
					</div>
					<div class="flex gap-2">
						{#each ['returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'lost'] as s}
							<button onclick={() => form.status = s} class="text-xs px-3 py-1 rounded border" class:bg-blue-700={form.status === s} class:text-white={form.status === s}>{labels[s]}</button>
						{/each}
					</div>
					{#if form.status === 'delayed'}
						<label class="block"><span class="text-xs text-neutral-600">Beräknad återlämning</span>
							<input type="date" bind:value={form.expectedReturnDate} oninput={() => checkConflict(g.name, form.expectedReturnDate)} class="block border rounded px-2 py-1 text-sm w-full" /></label>
						{#if delayWarning}<p class="text-xs text-orange-600">⚠ {delayWarning}</p>{/if}
					{/if}
					{#if form.status === 'reported_usable' || form.status === 'reported_unusable' || form.status === 'lost'}
						<label class="block"><span class="text-xs text-neutral-600">Beskrivning</span>
							<input type="text" bind:value={form.notes} placeholder="Vad hände?" class="block border rounded px-2 py-1 text-sm w-full" /></label>
					{/if}
					<div class="flex gap-2">
						<button onclick={() => confirmGroupForm(g)} disabled={!form.status || (form.status === 'delayed' && !form.expectedReturnDate)} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">Bekräfta</button>
						<button onclick={() => activeGroupKey = null} class="text-xs text-neutral-600 underline">Avbryt</button>
					</div>
				</div>
			{/if}
		{/if}
	{/each}

	{#each tracked as item}
		{@const notPicked = !item.pickup_status || item.pickup_status === 'lost'}
		{@const hasReturn = item.return_status && item.return_status !== 'pending'}
		<div class="border rounded px-4 py-3 flex items-center gap-3" class:bg-green-50={item.return_status === 'returned_ok'} class:bg-orange-50={item.return_status === 'delayed' || item.return_status === 'reported_usable'} class:bg-red-50={item.return_status === 'reported_unusable'} class:bg-challengerpink-50={item.return_status === 'lost'} class:bg-neutral-50={notPicked}>
			<div class="flex-1"><div class="font-medium text-sm">{item.common_name}</div><div class="text-xs text-neutral-500">{item.location_name}{item.place ? ` · ${item.place}` : ''}</div></div>
			{#if notPicked}
				<span class="text-xs text-neutral-400">Ej hämtad</span>
			{:else if hasReturn}
				<span class="text-xs px-2 py-0.5 rounded {colors[item.return_status] ?? ''}">{labels[item.return_status] ?? item.return_status}</span>
				{#if savedKey === item.id}<span class="text-xs text-green-600">Sparad</span>{/if}
				<button onclick={() => setReturn(item.id, '')} class="text-xs text-neutral-400 hover:text-neutral-600">Ångra</button>
			{:else if activeItemId !== item.id}
				<div class="flex gap-1">
					<button onclick={() => setReturn(item.id, 'returned_ok')} class="text-xs bg-green-700 text-white px-2 py-1 rounded">OK</button>
					<button onclick={() => openForm(item.id)} class="text-xs border px-2 py-1 rounded text-neutral-700">Annat...</button>
				</div>
			{/if}
		</div>
		{#if activeItemId === item.id}
			<div class="border rounded p-3 bg-neutral-50 text-sm space-y-2">
				<div class="flex gap-2">
					{#each ['returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'lost'] as s}
						<button onclick={() => form.status = s} class="text-xs px-3 py-1 rounded border" class:bg-blue-700={form.status === s} class:text-white={form.status === s}>{labels[s]}</button>
					{/each}
				</div>
				{#if form.status === 'delayed'}
					<label class="block"><span class="text-xs text-neutral-600">Beräknad återlämning</span>
						<input type="date" bind:value={form.expectedReturnDate} oninput={() => checkConflict(item.commercial_name, form.expectedReturnDate)} class="block border rounded px-2 py-1 text-sm w-full" /></label>
					{#if delayWarning}<p class="text-xs text-orange-600">⚠ {delayWarning}</p>{/if}
				{/if}
				{#if form.status === 'reported_usable' || form.status === 'reported_unusable' || form.status === 'lost'}
					<label class="block"><span class="text-xs text-neutral-600">Beskrivning</span>
						<input type="text" bind:value={form.notes} placeholder="Vad hände?" class="block border rounded px-2 py-1 text-sm w-full" /></label>
				{/if}
				<div class="flex gap-2">
					<button onclick={() => confirmForm(item.id)} disabled={!form.status || (form.status === 'delayed' && !form.expectedReturnDate)} class="text-xs bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">Bekräfta</button>
					<button onclick={() => activeItemId = null} class="text-xs text-neutral-600 underline">Avbryt</button>
				</div>
			</div>
		{/if}
	{/each}
</div>

{#if canComplete}
	<button onclick={() => { api.returnBooking(bookingId).then(onBookingReturned).catch((e) => error = e.message); }} class="mt-4 bg-green-700 text-white px-4 py-2 rounded text-sm">Slutför återlämning</button>
{/if}
