<script lang="ts">
	import { ApiError } from '$lib/api/client';
	import { translateError } from '$lib/errors';
	import * as m from '$lib/paraglide/messages.js';

	interface Item {
		id: string;
		name: string;
		[key: string]: any;
	}

	interface Props {
		items: Item[];
		label: string;
		placeholder: string;
		onCreate: (name: string) => Promise<Item>;
		onUpdate: (id: string, name: string) => Promise<Item>;
		onDelete: (id: string) => Promise<void>;
	}

	let { items = $bindable(), label, placeholder, onCreate, onUpdate, onDelete }: Props = $props();

	let editingId = $state<string | null>(null);
	let editingName = $state('');
	let newName = $state('');
	let error = $state('');
	let message = $state('');

	function flash(msg: string) {
		message = msg;
		setTimeout(() => message = '', 4000);
	}

	async function add() {
		if (!newName.trim()) return;
		error = '';
		try {
			const item = await onCreate(newName.trim());
			items = [...items, item];
			newName = '';
			flash(`${label} ${m.crud_added()}`);
		} catch (e: any) { error = translateError(e); }
	}

	async function save(id: string) {
		if (!editingName.trim()) return;
		error = '';
		try {
			const item = await onUpdate(id, editingName.trim());
			items = items.map(i => i.id === id ? item : i);
			editingId = null;
			flash(`${label} ${m.crud_updated()}`);
		} catch (e: any) { error = translateError(e); }
	}

	async function remove(id: string, name: string) {
		error = '';
		try {
			await onDelete(id);
			items = items.filter(i => i.id !== id);
			flash(`${name} ${m.crud_removed()}`);
		} catch (e: any) {
			if (e instanceof ApiError && e.body?.error === 'has_articles') {
				error = `${m.crud_delete_error_prefix()}${e.body.count} ${m.crud_delete_error_uses()} ${name.toLowerCase()}. ${m.crud_delete_error_hint()}`;
			} else {
				error = translateError(e);
			}
		}
	}
</script>

{#if message}
	<div class="bg-green-50 border border-green-200 rounded p-2 mb-2 text-green-800 text-sm">{message}</div>
{/if}
{#if error}
	<div class="bg-red-50 border border-red-200 rounded p-2 mb-2 text-red-800 text-sm">{error}</div>
{/if}
<div class="space-y-1 mb-2">
	{#each items as item}
		<div class="flex items-center gap-2 py-1">
			{#if editingId === item.id}
				<input type="text" bind:value={editingName} onkeydown={(e) => e.key === 'Enter' && save(item.id)} class="border rounded px-2 py-1 text-sm flex-1" />
				<button onclick={() => save(item.id)} class="text-xs text-blue-700 underline">{m.btn_save()}</button>
				<button onclick={() => editingId = null} class="text-xs text-neutral-500 underline">{m.btn_cancel()}</button>
			{:else}
				<span class="flex-1 text-sm">{item.name}</span>
				<button onclick={() => { editingId = item.id; editingName = item.name; }} class="text-xs text-blue-700 underline">{m.btn_edit()}</button>
				<button onclick={() => remove(item.id, item.name)} class="text-xs text-red-600 underline">{m.btn_delete()}</button>
			{/if}
		</div>
	{/each}
</div>
<div class="flex gap-2">
	<input type="text" bind:value={newName} placeholder={placeholder} onkeydown={(e) => e.key === 'Enter' && add()} class="border rounded px-2 py-1 text-sm flex-1" />
	<button onclick={add} disabled={!newName.trim()} class="text-sm bg-blue-700 text-white px-3 py-1 rounded disabled:opacity-50">{m.btn_add()}</button>
</div>
