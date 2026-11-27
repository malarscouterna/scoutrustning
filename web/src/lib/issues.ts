import type { Issue } from '#lib/api/client.ts';

export const ISSUE_STATUSES = ['open', 'in_progress', 'resolved', 'archived'] as const;
export const ISSUE_SEVERITIES = ['missing', 'unusable', 'usable'] as const;
export const DEFAULT_ISSUE_STATUSES: IssueStatus[] = ['open', 'in_progress'];
export const ISSUE_GROUPINGS = ['location', 'severity', 'status', 'assignee'] as const;
export const ISSUE_SORTS = ['oldest', 'newest', 'updated'] as const;
export const ISSUE_PRINT_FIELDS = [
	'checkbox',
	'id',
	'title',
	'status',
	'severity',
	'location',
	'assignee',
	'reporter',
	'date',
	'description'
] as const;
export const DEFAULT_ISSUE_PRINT_FIELDS: IssuePrintField[] = [
	'title',
	'status',
	'severity',
	'location',
	'assignee',
	'reporter',
	'date',
	'description'
];

export type IssueStatus = (typeof ISSUE_STATUSES)[number];
export type IssueSeverity = (typeof ISSUE_SEVERITIES)[number];
export type IssueGrouping = (typeof ISSUE_GROUPINGS)[number];
export type IssueSort = (typeof ISSUE_SORTS)[number];
export type IssuePrintField = (typeof ISSUE_PRINT_FIELDS)[number];

export interface IssueFilterState {
	statuses: IssueStatus[];
	severities: IssueSeverity[];
	locationIds: string[];
	search: string;
}

function csvValues(params: URLSearchParams, key: string): string[] {
	return params
		.getAll(key)
		.flatMap((value) => value.split(','))
		.map((value) => value.trim())
		.filter(Boolean);
}

function isOneOf<T extends string>(values: readonly T[], value: string): value is T {
	return (values as readonly string[]).includes(value);
}

function isUUID(value: string): boolean {
	return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
}

export function parseIssueFilters(params: URLSearchParams): IssueFilterState {
	let statuses: IssueStatus[];
	if (params.has('status')) {
		statuses = csvValues(params, 'status').filter(
			(value): value is IssueStatus => isOneOf(ISSUE_STATUSES, value)
		);
	} else if (params.get('closed') === 'true') {
		// Backwards compatibility for links/bookmarks using the old "show closed" toggle.
		statuses = [...ISSUE_STATUSES];
	} else {
		statuses = [...DEFAULT_ISSUE_STATUSES];
	}

	return {
		statuses,
		severities: csvValues(params, 'severity').filter(
			(value): value is IssueSeverity => isOneOf(ISSUE_SEVERITIES, value)
		),
		locationIds: csvValues(params, 'location').filter(isUUID),
		search: (params.get('search') ?? '').trim()
	};
}

export function parseIssuePrintFields(params: URLSearchParams): IssuePrintField[] {
	if (!params.has('fields')) return [...DEFAULT_ISSUE_PRINT_FIELDS];
	const selected = new Set(csvValues(params, 'fields'));
	return ISSUE_PRINT_FIELDS.filter((field) => selected.has(field));
}

export function parseIssueGrouping(value: string | null): IssueGrouping {
	return isOneOf(ISSUE_GROUPINGS, value ?? '') ? (value as IssueGrouping) : 'location';
}

export function parseIssueCompactMode(params: URLSearchParams): boolean {
	// `print=true` was the original parameter name; keep supporting old links and
	// bookmarks while new toggles write `compact=true`.
	return params.get('compact') === 'true' || params.get('print') === 'true';
}

export function parseIssueSort(value: string | null): IssueSort {
	return isOneOf(ISSUE_SORTS, value ?? '') ? (value as IssueSort) : 'oldest';
}

export function sortIssues(issues: Issue[], sort: IssueSort): Issue[] {
	const timestamp = (value: string) => new Date(value).getTime();
	const copy = [...issues];
	if (sort === 'newest') {
		copy.sort((a, b) => timestamp(b.created_at) - timestamp(a.created_at));
	} else if (sort === 'updated') {
		copy.sort((a, b) => timestamp(b.updated_at) - timestamp(a.updated_at));
	} else {
		copy.sort((a, b) => timestamp(a.created_at) - timestamp(b.created_at));
	}
	return copy;
}
