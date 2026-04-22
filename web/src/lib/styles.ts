export const articleStatusColors: Record<string, string> = {
	ok: 'bg-green-100 text-green-800',
	reported_usable: 'bg-orange-100 text-orange-800',
	incoming: 'bg-blue-50 text-blue-700',
	reported_unusable: 'bg-red-100 text-red-800',
	under_repair: 'bg-neutral-100 text-neutral-700',
	lost: 'bg-challengerpink-100 text-challengerpink-800',
	archived: 'bg-neutral-100 text-neutral-500'
};

export const bookingStatusColors: Record<string, string> = {
	draft: 'bg-neutral-100',
	submitted: 'bg-orange-100 text-orange-800',
	approved: 'bg-blue-100 text-blue-800',
	confirmed: 'bg-green-100 text-green-800',
	picked_up: 'bg-blue-100 text-blue-800',
	returned: 'bg-neutral-100',
	rejected: 'bg-red-100 text-red-800',
	cancelled: 'bg-neutral-100 text-neutral-500'
};

export const bookingStatusLeftBorder: Record<string, string> = {
	submitted: 'border-l-4 border-l-orange-400',
	approved: 'border-l-4 border-l-blue-500',
	confirmed: 'border-l-4 border-l-blue-500',
	picked_up: 'border-l-4 border-l-blue-500'
};

export const issueStatusColors: Record<string, string> = {
	open: 'bg-blue-50 text-blue-700',
	in_progress: 'bg-yellow-50 text-yellow-800',
	resolved: 'bg-green-100 text-green-800',
	archived: 'bg-neutral-100 text-neutral-500'
};

export const issueSeverityColors: Record<string, string> = {
	usable: 'bg-orange-100 text-orange-800',
	unusable: 'bg-red-100 text-red-800',
	missing: 'bg-red-100 text-red-800'
};

export const articleEventTypeColors: Record<string, string> = {
	issue_reported: 'text-orange-700',
	issue_resolved: 'text-green-700',
	status_change: 'text-blue-700',
	count_changed: 'text-purple-700',
	returned: 'text-green-700'
};
