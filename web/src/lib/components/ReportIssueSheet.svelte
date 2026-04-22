<script lang="ts">
	import { createApiClient, type IssueDetail } from '$lib/api/client';
	import ImageAttachInput from '$lib/components/ImageAttachInput.svelte';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		articleId: string;
		articleName: string;
		open: boolean;
		severity?: string;
		bookingId?: string;
		isQuantityTracked?: boolean;
		groupTotal?: number;
		onReported?: (issue: IssueDetail) => void;
		onClose?: () => void;
	}

	let {
		articleId,
		articleName,
		open = $bindable(),
		severity = $bindable('unusable'),
		bookingId,
		isQuantityTracked = false,
		groupTotal = 0,
		onReported,
		onClose
	}: Props = $props();

	const api = createApiClient();

	let description = $state('');
	let imageIds = $state<string[]>([]);
	let count = $state(1);
	let submitting = $state(false);
	let error = $state('');

	const options = $derived([
		{ value: 'usable', label: m.issue_severity_usable(), sub: m.report_issue_severity_usable_desc() },
		{ value: 'unusable', label: m.issue_severity_unusable(), sub: m.report_issue_severity_unusable_desc() },
		{ value: 'missing', label: m.issue_severity_missing(), sub: m.report_issue_severity_missing_desc() }
	]);

	function close() {
		open = false;
		description = '';
		imageIds = [];
		count = 1;
		error = '';
		onClose?.();
	}

	async function submit() {
		if (!description.trim()) { error = m.report_issue_describe_error(); return; }
		error = '';
		submitting = true;
		try {
			const issue = await api.createIssue({
				article_id: articleId,
				severity,
				description: description.trim(),
				...(bookingId ? { booking_id: bookingId } : {}),
				...(imageIds.length ? { image_ids: imageIds } : {}),
				...(isQuantityTracked && count > 1 ? { count } : {})
			});
			onReported?.(issue);
			close();
		} catch (e: any) {
			error = e.message ?? m.common_error();
		}
		submitting = false;
	}
</script>

{#if open}
	<!-- Backdrop -->
	<button
		type="button"
		class="fixed inset-0 z-40 bg-black/60"
		aria-label={m.btn_close()}
		onclick={close}
	></button>

	<!-- Centered modal -->
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
	<div
		class="bg-white rounded-2xl shadow-xl w-full max-w-md max-h-[calc(100dvh-2rem)] overflow-y-auto p-5 space-y-4 pointer-events-auto"
		role="dialog"
		aria-modal="true"
		aria-label={m.report_issue_title()}
	>
		<div class="flex items-start justify-between gap-2">
			<p class="font-semibold text-sm">{m.report_issue_title()} – {articleName}</p>
			<button type="button" onclick={close} aria-label={m.btn_close()} class="text-neutral-400 hover:text-neutral-700 shrink-0 text-lg leading-none">×</button>
		</div>

		{#if error}
			<p class="text-red-600 text-xs">{error}</p>
		{/if}

		{#if isQuantityTracked}
			<div>
				<label for="sheet-count" class="text-xs font-medium text-neutral-500 block mb-1">
					{m.report_issue_count_label()}
					{#if groupTotal > 0}<span class="text-neutral-400 font-normal">({m.common_of()} {groupTotal} st)</span>{/if}
				</label>
				<input
					id="sheet-count"
					type="number"
					bind:value={count}
					min="1"
					max={groupTotal > 0 ? groupTotal : undefined}
					class="w-24 border rounded px-3 py-2 text-sm"
				/>
			</div>
		{/if}

		<fieldset>
			<legend class="text-xs font-medium text-neutral-500 mb-2">{m.report_issue_severity_legend()}</legend>
			<div class="space-y-2">
				{#each options as opt}
					<label class="flex items-center gap-3 cursor-pointer">
						<input type="radio" bind:group={severity} value={opt.value} class="mt-0.5" />
						<span class="text-sm">
							<span class="font-medium">{opt.label}</span>
							<span class="text-neutral-500"> – {opt.sub}</span>
						</span>
					</label>
				{/each}
			</div>
		</fieldset>

		<div>
			<label for="sheet-description" class="text-xs font-medium text-neutral-500 block mb-1">
				{m.lbl_description()} <span class="text-red-500">*</span>
			</label>
			<textarea
				id="sheet-description"
				bind:value={description}
				rows="3"
				placeholder={m.report_issue_description_placeholder()}
				class="w-full border rounded px-3 py-2 text-sm"
			></textarea>
		</div>

		<div>
			<p class="text-xs font-medium text-neutral-500 mb-1">{m.lbl_images()} <span class="text-neutral-400 font-normal">{m.optional()}</span></p>
			<ImageAttachInput bind:imageIds />
		</div>

		<div class="flex gap-3">
			<button
				onclick={submit}
				disabled={submitting || !description.trim()}
				class="flex-1 bg-blue-700 text-white text-sm font-medium py-2.5 rounded disabled:opacity-50"
			>
				{submitting ? m.btn_submitting() : m.report_issue_submit()}
			</button>
			<button onclick={close} class="text-sm text-neutral-600 underline px-2">{m.btn_cancel()}</button>
		</div>
	</div>
	</div>
{/if}
