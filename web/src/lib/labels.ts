export const statusLabels: Record<string, string> = {
	ok: 'OK',
	reported_usable: 'Felrapporterad — användbar',
	incoming: 'Inkommande',
	reported_unusable: 'Felrapporterad — ej användbar',
	under_repair: 'Under reparation',
	lost: 'Saknas',
	archived: 'Arkiverad'
};

export const statusColors: Record<string, string> = {
	ok: 'bg-green-100 text-green-800',
	reported_usable: 'bg-orange-100 text-orange-800',
	incoming: 'bg-blue-50 text-blue-700',
	reported_unusable: 'bg-red-100 text-red-800',
	under_repair: 'bg-neutral-100 text-neutral-700',
	lost: 'bg-challengerpink-100 text-challengerpink-800',
	archived: 'bg-neutral-100 text-neutral-500'
};

export const approvalLabels: Record<string, string> = {
	none: 'Ingen',
	low: 'Låg',
	high: 'Hög'
};

export const eventTypeLabels: Record<string, string> = {
	issue_reported: 'Problem rapporterat',
	issue_resolved: 'Problem löst',
	status_change: 'Statusändring',
	count_changed: 'Antal ändrat',
	returned: 'Återlämnad',
	booked: 'Bokad',
	picked_up: 'Uthämtad',
	note: 'Kommentar'
};

export const eventTypeColors: Record<string, string> = {
	issue_reported: 'text-orange-700',
	issue_resolved: 'text-green-700',
	status_change: 'text-blue-700',
	count_changed: 'text-purple-700',
	returned: 'text-green-700'
};
