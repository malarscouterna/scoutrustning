export const ISSUE_STATUSES = ['open', 'in_progress', 'resolved', 'archived'] as const;
export const ISSUE_SEVERITIES = ['missing', 'unusable', 'usable'] as const;
export const DEFAULT_ISSUE_STATUSES: IssueStatus[] = ['open', 'in_progress'];
export const ISSUE_GROUPINGS = ['location', 'severity', 'status', 'assignee'] as const;
export const ISSUE_SORTS = ['oldest', 'newest', 'updated'] as const;

export type IssueStatus = (typeof ISSUE_STATUSES)[number];
export type IssueSeverity = (typeof ISSUE_SEVERITIES)[number];
export type IssueGrouping = (typeof ISSUE_GROUPINGS)[number];
export type IssueSort = (typeof ISSUE_SORTS)[number];

export interface IssueFilterState {
	statuses: IssueStatus[];
	severities: IssueSeverity[];
	locationIds: string[];
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
		locationIds: csvValues(params, 'location').filter(isUUID)
	};
}

export function parseIssueGrouping(value: string | null): IssueGrouping {
	return isOneOf(ISSUE_GROUPINGS, value ?? '') ? (value as IssueGrouping) : 'location';
}

export function parseIssueSort(value: string | null): IssueSort {
	return isOneOf(ISSUE_SORTS, value ?? '') ? (value as IssueSort) : 'oldest';
}
