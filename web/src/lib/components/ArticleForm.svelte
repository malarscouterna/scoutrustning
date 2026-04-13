<script lang="ts">
	import type { Location, Category } from '$lib/api/client';

	interface Props {
		locations: Location[];
		categories: Category[];
		mode: 'create' | 'edit';
		isManager?: boolean;
		userName?: string;
		userGroup?: string;
		individuallyTrackedEdit?: boolean;
		quantityTrackedEdit?: boolean;
		groupCount?: number | null;
		initial?: {
			commercial_name?: string;
			common_name?: string;
			category_id?: string;
			location_id?: string;
			status?: string;
			individually_tracked?: boolean;
			approval_level?: string;
			description?: string;
			instructions?: string;
			place?: string;
			purchase_date?: string | null;
			purchase_price?: string | null;
			manager_notes?: string;
			image_ids?: string[];
		};
		submitLabel?: string;
		onSubmit: (articles: Record<string, unknown>[]) => Promise<void>;
		onCancel?: () => void;
		error?: string;
		saving?: boolean;
	}

	let { locations, categories, mode, isManager = false, userName = '', userGroup = '', individuallyTrackedEdit = false, quantityTrackedEdit = false, groupCount = null, initial, submitLabel = 'Spara', onSubmit, onCancel, error = '', saving = false }: Props = $props();

	let countValue = $state(0);
	let countSaving = $state(false);

	$effect(() => {
		if (groupCount !== null) countValue = groupCount;
	});

	let form = $state({
		commercial_name: '',
		common_name: '',
		category_id: '',
		location_id: '',
		status: 'ok',
		individually_tracked: true,
		approval_level: 'none',
		description: '',
		instructions: '',
		place: '',
		purchase_date: '',
		purchase_price: '',
		manager_notes: ''
	});

	let imageIds = $state<string[]>([]);

	$effect(() => {
		if (!initial) return;
		imageIds = Array.isArray(initial.image_ids) ? initial.image_ids : [];
		form.commercial_name = initial.commercial_name ?? '';
		form.common_name = initial.common_name ?? '';
		form.category_id = initial.category_id ?? '';
		form.location_id = initial.location_id ?? '';
		form.status = initial.status ?? 'ok';
		form.individually_tracked = initial.individually_tracked ?? true;
		form.approval_level = initial.approval_level ?? 'none';
		form.description = initial.description ?? '';
		form.instructions = initial.instructions ?? '';
		form.place = initial.place ?? '';
		form.purchase_date = initial.purchase_date ?? '';
		form.purchase_price = initial.purchase_price ?? '';
		form.manager_notes = initial.manager_notes ?? '';
	});

	// Multi-create state
	let count = $state(1);
	let names = $state<string[]>([]);
	let lastBase = $state('');

	$effect(() => {
		if (mode !== 'create' || !form.individually_tracked) return;
		const base = form.commercial_name || 'Artikel';
		const baseChanged = base !== lastBase;
		if (baseChanged || names.length !== count) {
			const next: string[] = [];
			for (let i = 0; i < count; i++) {
				// Keep user-edited names when only count changed
				if (!baseChanged && i < names.length) {
					next.push(names[i]);
				} else {
					next.push(`${base} ${i + 1}`);
				}
			}
			names = next;
			lastBase = base;
		}
	});

	function updateName(index: number, value: string) {
		const next = [...names];
		next[index] = value;
		names = next;
	}

	function buildBase(): Record<string, unknown> {
		const data: Record<string, unknown> = {
			commercial_name: form.commercial_name,
			category_id: form.category_id,
			location_id: form.location_id,
			status: form.status,
			individually_tracked: form.individually_tracked,
			approval_level: form.approval_level,
			description: form.description,
			instructions: form.instructions,
			place: form.place
		};
		if (form.purchase_date) data.purchase_date = form.purchase_date;
		if (form.purchase_price) data.purchase_price = form.purchase_price;
		if (form.manager_notes) data.manager_notes = form.manager_notes;
		return data;
	}

	async function handleSubmit() {
		if (mode === 'edit') {
			const data = buildBase();
			data.common_name = form.common_name;
			if (quantityTrackedEdit && groupCount !== null && countValue !== groupCount) {
				data._newCount = countValue;
			}
			await onSubmit([data]);
			return;
		}

		// Create mode
		const base = buildBase();
		if (form.individually_tracked) {
			const articles = names.map(name => ({ ...base, common_name: name }));
			await onSubmit(articles);
		} else {
			// Quantity tracked: create `count` articles with same common_name
			const articles = Array.from({ length: count }, () => ({
				...base,
				common_name: form.commercial_name || form.common_name || 'Artikel',
				individually_tracked: false
			}));
			await onSubmit(articles);
		}
	}

	const statusOptions = [
		{ value: 'ok', label: 'OK' },
		{ value: 'reported_usable', label: 'Felrapporterad — användbar' },
		{ value: 'incoming', label: 'Inkommande' },
		{ value: 'reported_unusable', label: 'Felrapporterad — ej användbar' },
		{ value: 'under_repair', label: 'Under reparation' },
		{ value: 'lost', label: 'Saknas' },
		{ value: 'archived', label: 'Arkiverad' }
	];

	const approvalOptions = [
		{ value: 'none', label: 'Ingen (fritt bokbar)' },
		{ value: 'low', label: 'Låg (projektledare godkänner)' },
		{ value: 'high', label: 'Hög (alltid godkännande)' }
	];

	let valid = $derived(
		form.category_id && form.location_id &&
		(mode === 'edit' ? !!form.common_name : (form.individually_tracked ? names.every(n => n.trim()) : count > 0))
	);

	let nameWarning = $derived(
		mode === 'edit' && form.commercial_name && form.common_name
		&& !form.common_name.startsWith(form.commercial_name)
	);
</script>

<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4">
	{#if error}
		<div class="bg-red-50 border border-red-200 rounded p-2 text-red-800 text-sm">{error}</div>
	{/if}

	{#if individuallyTrackedEdit}
		<div class="border border-blue-200 bg-blue-50/30 rounded p-4 space-y-4">
			<p class="text-xs text-blue-600">Gemensamt — uppdateras för alla artiklar i gruppen</p>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Produktnamn</span>
				<input type="text" bind:value={form.commercial_name} placeholder="t.ex. Sibley" class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Kategori *</span>
				<select bind:value={form.category_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">Välj...</option>
					{#each categories as cat}
						<option value={cat.id}>{cat.name}</option>
					{/each}
				</select>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
				<textarea bind:value={form.description} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Instruktioner</span>
				<textarea bind:value={form.instructions} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			{#if isManager}
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Interna anteckningar (bara synligt för utrustningsansvariga)</span>
					<textarea bind:value={form.manager_notes} rows="2" placeholder="Interna noteringar..." class="border rounded px-2 py-1.5 text-sm w-full bg-amber-50"></textarea>
				</label>
			{/if}
		</div>

		<div class="border border-neutral-200 rounded p-4 space-y-4">
			<p class="text-xs text-neutral-500">Enskild artikel — gäller bara denna</p>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Artikelnamn *</span>
				<input type="text" bind:value={form.common_name} required placeholder="t.ex. Sibley 1" class="border rounded px-2 py-1.5 text-sm w-full" />
				{#if nameWarning}
					<p class="text-xs text-amber-600 mt-1">Artikelnamnet börjar inte med produktnamnet ({form.commercial_name})</p>
				{/if}
			</label>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Status</span>
					<select bind:value={form.status} class="border rounded px-2 py-1.5 text-sm w-full">
						{#each statusOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Godkännandenivå</span>
					<select bind:value={form.approval_level} class="border rounded px-2 py-1.5 text-sm w-full">
						{#each approvalOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
			</div>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Plats *</span>
				<select bind:value={form.location_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">Välj...</option>
					{#each locations as loc}
						<option value={loc.id}>{loc.name}</option>
					{/each}
				</select>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Förvaringsplats</span>
				<input type="text" bind:value={form.place} placeholder="t.ex. Hylla 3" class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Inköpsdatum</span>
					<input type="date" bind:value={form.purchase_date} class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Inköpspris (kr)</span>
					<input type="number" bind:value={form.purchase_price} step="0.01" min="0" class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
			</div>
		</div>
	{:else if quantityTrackedEdit}
		<div class="border border-blue-200 bg-blue-50/30 rounded p-4 space-y-4">

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Produktnamn</span>
				<input type="text" bind:value={form.commercial_name} placeholder="t.ex. Sibley" class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Kategori *</span>
					<select bind:value={form.category_id} required class="border rounded px-2 py-1.5 text-sm w-full">
						<option value="">Välj...</option>
						{#each categories as cat}
							<option value={cat.id}>{cat.name}</option>
						{/each}
					</select>
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Godkännandenivå</span>
					<select bind:value={form.approval_level} class="border rounded px-2 py-1.5 text-sm w-full">
						{#each approvalOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
			</div>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Plats *</span>
					<select bind:value={form.location_id} required class="border rounded px-2 py-1.5 text-sm w-full">
						<option value="">Välj...</option>
						{#each locations as loc}
							<option value={loc.id}>{loc.name}</option>
						{/each}
					</select>
				</label>
				{#if groupCount !== null}
					<label class="block">
						<span class="text-sm text-neutral-600 block mb-1">Antal</span>
						<input type="number" bind:value={countValue} min="0" max="999" class="border rounded px-2 py-1.5 text-sm w-full" />
					</label>
				{/if}
			</div>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Förvaringsplats</span>
				<input type="text" bind:value={form.place} placeholder="t.ex. Hylla 3" class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
				<textarea bind:value={form.description} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Instruktioner</span>
				<textarea bind:value={form.instructions} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			{#if isManager}
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">Interna anteckningar (bara synligt för utrustningsansvariga)</span>
					<textarea bind:value={form.manager_notes} rows="2" placeholder="Interna noteringar..." class="border rounded px-2 py-1.5 text-sm w-full bg-amber-50"></textarea>
				</label>
			{/if}
		</div>
	{:else}
		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">Produktnamn</span>
			<input type="text" bind:value={form.commercial_name} placeholder="t.ex. Sibley" class="border rounded px-2 py-1.5 text-sm w-full" />
		</label>

		{#if mode === 'edit'}
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Artikelnamn *</span>
				<input type="text" bind:value={form.common_name} required placeholder="t.ex. Sibley 1" class="border rounded px-2 py-1.5 text-sm w-full" />
				{#if nameWarning}
					<p class="text-xs text-amber-600 mt-1">Artikelnamnet börjar inte med produktnamnet ({form.commercial_name})</p>
				{/if}
			</label>
		{/if}

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Kategori *</span>
				<select bind:value={form.category_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">Välj...</option>
					{#each categories as cat}
						<option value={cat.id}>{cat.name}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Godkännandenivå</span>
				<select bind:value={form.approval_level} class="border rounded px-2 py-1.5 text-sm w-full">
					{#each approvalOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</label>
		</div>

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Plats *</span>
				<select bind:value={form.location_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">Välj...</option>
					{#each locations as loc}
						<option value={loc.id}>{loc.name}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Status</span>
				<select bind:value={form.status} class="border rounded px-2 py-1.5 text-sm w-full">
					{#each statusOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</label>
		</div>

		{#if mode === 'create'}
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={form.individually_tracked} />
				Individuellt spårad
			</label>
		{/if}

		{#if mode === 'create'}
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Antal</span>
				<input type="number" bind:value={count} min="1" max="200" class="border rounded px-2 py-1.5 text-sm w-24" />
			</label>

			{#if form.individually_tracked && count > 0}
				<div>
					<span class="text-sm text-neutral-600 block mb-1">Artikelnamn ({count} st)</span>
					<div class="space-y-1">
						{#each names as name, i}
							<input
								type="text"
								value={name}
								oninput={(e) => updateName(i, (e.target as HTMLInputElement).value)}
								placeholder="Artikelnamn {i + 1}"
								class="border rounded px-2 py-1.5 text-sm w-full"
							/>
						{/each}
					</div>
				</div>
			{/if}
		{/if}

		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">Förvaringsplats</span>
			<input type="text" bind:value={form.place} placeholder="t.ex. Hylla 3" class="border rounded px-2 py-1.5 text-sm w-full" />
		</label>

		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">Beskrivning</span>
			<textarea bind:value={form.description} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
		</label>

		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">Instruktioner</span>
			<textarea bind:value={form.instructions} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
		</label>

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Inköpsdatum</span>
				<input type="date" bind:value={form.purchase_date} class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Inköpspris per styck (kr)</span>
				<input type="number" bind:value={form.purchase_price} step="0.01" min="0" class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>
		</div>

		{#if isManager}
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">Interna anteckningar (bara synligt för utrustningsansvariga)</span>
				<textarea bind:value={form.manager_notes} rows="2" placeholder="Interna noteringar..." class="border rounded px-2 py-1.5 text-sm w-full bg-amber-50"></textarea>
			</label>
		{/if}
	{/if}

	<div class="flex gap-2 pt-2">
		<button type="submit" disabled={saving || !valid} class="bg-blue-700 text-white px-4 py-2 rounded text-sm disabled:opacity-50">
			{saving ? 'Sparar...' : submitLabel}
		</button>
		{#if onCancel}
			<button type="button" onclick={onCancel} class="text-sm text-neutral-500 underline">Avbryt</button>
		{/if}
	</div>
</form>
