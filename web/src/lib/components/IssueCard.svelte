<script lang="ts">
	import type { Issue } from '$lib/api/client';

	interface Props {
		issue: Issue;
	}

	let { issue }: Props = $props();

	const severityLabels: Record<string, string> = {
		usable: 'Användbar',
		unusable: 'Ej användbar',
		missing: 'Saknas'
	};

	const severityColors: Record<string, string> = {
		usable: 'bg-orange-100 text-orange-800',
		unusable: 'bg-red-100 text-red-800',
		missing: 'bg-red-100 text-red-800'
	};

	const statusLabels: Record<string, string> = {
		open: 'Öppen',
		in_progress: 'Pågår',
		resolved: 'Löst',
		archived: 'Arkiverad'
	};

	const statusColors: Record<string, string> = {
		open: 'bg-blue-50 text-blue-700',
		in_progress: 'bg-yellow-50 text-yellow-800',
		resolved: 'bg-green-100 text-green-800',
		archived: 'bg-neutral-100 text-neutral-500'
	};

	function formatDate(ts: string) {
		const d = new Date(ts);
		const now = new Date();
		if (
			d.getDate() === now.getDate() &&
			d.getMonth() === now.getMonth() &&
			d.getFullYear() === now.getFullYear()
		) return 'idag';
		if (now.getTime() - d.getTime() < 172800000) return 'igår';
		return d.toLocaleDateString('sv', {
			day: 'numeric',
			month: 'short',
			year: d.getFullYear() !== now.getFullYear() ? 'numeric' : undefined
		});
	}

	let articleNames = $derived.by(() => {
		// Group quantity-tracked articles by commercial_name+location and show count
		const parts: string[] = [];
		const seen = new Map<string, number>();
		for (const a of issue.articles) {
			if (!a.individually_tracked) {
				const key = a.commercial_name + '|' + a.location_name;
				seen.set(key, (seen.get(key) ?? 0) + 1);
			} else {
				parts.push(a.commercial_name + (a.common_name ? ' \u2013 ' + a.common_name : ''));
			}
		}
		for (const [key, count] of seen) {
			const name = key.split('|')[0];
			parts.push(count > 1 ? `${name} (${count} st)` : name);
		}
		return parts.join(', ');
	});
</script>

<a href="/issues/{issue.id}" class="block border rounded px-4 py-3 hover:bg-neutral-50 space-y-1.5">
	<div class="flex flex-wrap items-start justify-between gap-2">
		<span class="font-medium text-sm">{issue.title}</span>
		{#if issue.status !== 'open'}
			<span class="text-xs px-2 py-0.5 rounded font-medium shrink-0 {statusColors[issue.status] ?? 'bg-neutral-100'}">
				{statusLabels[issue.status] ?? issue.status}
			</span>
		{/if}
	</div>
	<div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-neutral-500">
		<span class="px-1.5 py-0.5 rounded font-medium {severityColors[issue.severity] ?? 'bg-neutral-100'}">
			{severityLabels[issue.severity] ?? issue.severity}
		</span>
		{#if articleNames}
			<span>{articleNames}</span>
		{/if}
	</div>
	<p class="text-xs text-neutral-400">
		Rapporterat {formatDate(issue.created_at)} av {issue.reporter_name}
		{#if issue.updated_at !== issue.created_at}
			· uppdaterat {formatDate(issue.updated_at)}
		{/if}
	</p>
</a>
