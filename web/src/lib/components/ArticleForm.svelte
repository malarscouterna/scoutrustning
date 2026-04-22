<script lang="ts">
	import type { Location, Category } from '$lib/api/client';
	import * as m from '$lib/paraglide/messages.js';

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

	let { locations, categories, mode, isManager = false, userName = '', userGroup = '', individuallyTrackedEdit = false, quantityTrackedEdit = false, groupCount = null, initial, submitLabel, onSubmit, onCancel, error = '', saving = false }: Props = $props();
	let resolvedSubmitLabel = $derived(submitLabel ?? m.btn_save());

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

	let statusOptions = $derived([
		{ value: 'ok', label: m.article_status_ok() },
		{ value: 'reported_usable', label: m.article_status_reported_usable() },
		{ value: 'incoming', label: m.article_status_incoming() },
		{ value: 'reported_unusable', label: m.article_status_reported_unusable() },
		{ value: 'under_repair', label: m.article_status_under_repair() },
		{ value: 'lost', label: m.article_status_lost() },
		{ value: 'archived', label: m.article_status_archived() }
	]);

	let approvalOptions = $derived([
		{ value: 'none', label: m.article_form_approval_none_label() },
		{ value: 'low', label: m.article_form_approval_low_label() },
		{ value: 'high', label: m.article_form_approval_high_label() }
	]);

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
			<p class="text-xs text-blue-600">{m.article_form_shared_hint()}</p>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_product_name()}</span>
				<input type="text" bind:value={form.commercial_name} placeholder={m.article_form_product_name_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_category()}</span>
				<select bind:value={form.category_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">{m.common_select_placeholder()}</option>
					{#each categories as cat}
						<option value={cat.id}>{cat.name}</option>
					{/each}
				</select>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.lbl_description()}</span>
				<textarea bind:value={form.description} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.lbl_instructions()}</span>
				<textarea bind:value={form.instructions} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			{#if isManager}
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_internal_notes_label()}</span>
					<textarea bind:value={form.manager_notes} rows="2" placeholder={m.article_form_internal_notes_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full bg-amber-50"></textarea>
				</label>
			{/if}
		</div>

		<div class="border border-neutral-200 rounded p-4 space-y-4">
			<p class="text-xs text-neutral-500">{m.article_form_individual_hint()}</p>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_article_name()}</span>
				<input type="text" bind:value={form.common_name} required placeholder={m.article_form_article_name_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
				{#if nameWarning}
					<p class="text-xs text-amber-600 mt-1">{m.article_form_name_warning()} ({form.commercial_name})</p>
				{/if}
			</label>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.lbl_status()}</span>
					<select bind:value={form.status} class="border rounded px-2 py-1.5 text-sm w-full">
						{#each statusOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_approval_level()}</span>
					<select bind:value={form.approval_level} class="border rounded px-2 py-1.5 text-sm w-full">
						{#each approvalOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
			</div>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_location()}</span>
				<select bind:value={form.location_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">{m.common_select_placeholder()}</option>
					{#each locations as loc}
						<option value={loc.id}>{loc.name}</option>
					{/each}
				</select>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_storage_location()}</span>
				<input type="text" bind:value={form.place} placeholder={m.article_form_storage_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_purchase_date()}</span>
					<input type="date" bind:value={form.purchase_date} class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_purchase_price()}</span>
					<input type="number" bind:value={form.purchase_price} step="0.01" min="0" class="border rounded px-2 py-1.5 text-sm w-full" />
				</label>
			</div>
		</div>
	{:else if quantityTrackedEdit}
		<div class="border border-blue-200 bg-blue-50/30 rounded p-4 space-y-4">

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_product_name()}</span>
				<input type="text" bind:value={form.commercial_name} placeholder={m.article_form_product_name_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_category()}</span>
					<select bind:value={form.category_id} required class="border rounded px-2 py-1.5 text-sm w-full">
						<option value="">{m.common_select_placeholder()}</option>
						{#each categories as cat}
							<option value={cat.id}>{cat.name}</option>
						{/each}
					</select>
				</label>
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_approval_level()}</span>
					<select bind:value={form.approval_level} class="border rounded px-2 py-1.5 text-sm w-full">
						{#each approvalOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</label>
			</div>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_location()}</span>
					<select bind:value={form.location_id} required class="border rounded px-2 py-1.5 text-sm w-full">
						<option value="">{m.common_select_placeholder()}</option>
						{#each locations as loc}
							<option value={loc.id}>{loc.name}</option>
						{/each}
					</select>
				</label>
				{#if groupCount !== null}
					<label class="block">
						<span class="text-sm text-neutral-600 block mb-1">{m.article_form_count()}</span>
						<input type="number" bind:value={countValue} min="0" max="999" class="border rounded px-2 py-1.5 text-sm w-full" />
					</label>
				{/if}
			</div>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_storage_location()}</span>
				<input type="text" bind:value={form.place} placeholder={m.article_form_storage_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.lbl_description()}</span>
				<textarea bind:value={form.description} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.lbl_instructions()}</span>
				<textarea bind:value={form.instructions} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
			</label>

			{#if isManager}
				<label class="block">
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_internal_notes_label()}</span>
					<textarea bind:value={form.manager_notes} rows="2" placeholder={m.article_form_internal_notes_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full bg-amber-50"></textarea>
				</label>
			{/if}
		</div>
	{:else}
		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">{m.article_form_product_name()}</span>
			<input type="text" bind:value={form.commercial_name} placeholder={m.article_form_product_name_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
		</label>

		{#if mode === 'edit'}
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_article_name()}</span>
				<input type="text" bind:value={form.common_name} required placeholder={m.article_form_article_name_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
				{#if nameWarning}
					<p class="text-xs text-amber-600 mt-1">{m.article_form_name_warning()} ({form.commercial_name})</p>
				{/if}
			</label>
		{/if}

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_category()}</span>
				<select bind:value={form.category_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">{m.common_select_placeholder()}</option>
					{#each categories as cat}
						<option value={cat.id}>{cat.name}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_approval_level()}</span>
				<select bind:value={form.approval_level} class="border rounded px-2 py-1.5 text-sm w-full">
					{#each approvalOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</label>
		</div>

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_location()}</span>
				<select bind:value={form.location_id} required class="border rounded px-2 py-1.5 text-sm w-full">
					<option value="">{m.common_select_placeholder()}</option>
					{#each locations as loc}
						<option value={loc.id}>{loc.name}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.lbl_status()}</span>
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
				{m.article_form_individually_tracked()}
			</label>
		{/if}

		{#if mode === 'create'}
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_count()}</span>
				<input type="number" bind:value={count} min="1" max="200" class="border rounded px-2 py-1.5 text-sm w-24" />
			</label>

			{#if form.individually_tracked && count > 0}
				<div>
					<span class="text-sm text-neutral-600 block mb-1">{m.article_form_article_name()} ({count} {m.common_unit_count()})</span>
					<div class="space-y-1">
						{#each names as name, i}
							<input
								type="text"
								value={name}
								oninput={(e) => updateName(i, (e.target as HTMLInputElement).value)}
								placeholder="{m.article_form_article_name()} {i + 1}"
								class="border rounded px-2 py-1.5 text-sm w-full"
							/>
						{/each}
					</div>
				</div>
			{/if}
		{/if}

		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">{m.article_form_storage_location()}</span>
			<input type="text" bind:value={form.place} placeholder={m.article_form_storage_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full" />
		</label>

		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">{m.lbl_description()}</span>
			<textarea bind:value={form.description} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
		</label>

		<label class="block">
			<span class="text-sm text-neutral-600 block mb-1">{m.lbl_instructions()}</span>
			<textarea bind:value={form.instructions} rows="2" class="border rounded px-2 py-1.5 text-sm w-full"></textarea>
		</label>

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_purchase_date()}</span>
				<input type="date" bind:value={form.purchase_date} class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_purchase_price_each()}</span>
				<input type="number" bind:value={form.purchase_price} step="0.01" min="0" class="border rounded px-2 py-1.5 text-sm w-full" />
			</label>
		</div>

		{#if isManager}
			<label class="block">
				<span class="text-sm text-neutral-600 block mb-1">{m.article_form_internal_notes_label()}</span>
				<textarea bind:value={form.manager_notes} rows="2" placeholder={m.article_form_internal_notes_placeholder()} class="border rounded px-2 py-1.5 text-sm w-full bg-amber-50"></textarea>
			</label>
		{/if}
	{/if}

	<div class="flex gap-2 pt-2">
		<button type="submit" disabled={saving || !valid} class="bg-blue-700 text-white px-4 py-2 rounded text-sm disabled:opacity-50">
			{saving ? m.btn_saving() : resolvedSubmitLabel}
		</button>
		{#if onCancel}
			<button type="button" onclick={onCancel} class="text-sm text-neutral-500 underline">{m.btn_cancel()}</button>
		{/if}
	</div>
</form>
