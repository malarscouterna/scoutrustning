import { goto } from '$app/navigation';
import { page } from '$app/state';

// Issue filters, search, sorting, grouping and field selection all live in the
// URL. Shallow routing stores its target in `page.shallow.url`, so all changes
// build from `currentIssueUrl()`. SvelteKit does not commit `page.url` until a
// navigation finishes, and a newer `goto` aborts an older one. Build every
// change from the latest intended URL instead, so overlapping changes are not
// silently dropped.
let pendingUrl: URL | null = null;
let pendingRequiresLoad = false;
let queue: Promise<void> = Promise.resolve();
let generation = 0;

function isAbort(error: unknown): boolean {
	return error instanceof Error && (error.name === 'AbortError' || /aborted/i.test(error.message));
}

// `goto(..., { shallow: true })` updates `page.shallow.url`, not `page.url`.
// Use the shallow target when present so presentation params survive later loads.
export function currentIssueUrl(): URL {
	return new URL((page.shallow?.url ?? page.url).href);
}

export function updateIssueUrl(
	mutate: (params: URLSearchParams) => void,
	options: { replace?: boolean; shallow?: boolean } = {}
): Promise<void> {
	const base = pendingUrl ?? currentIssueUrl();
	const next = new URL(base.href);
	mutate(next.searchParams);
	if (next.href === base.href) return Promise.resolve();
	// A shallow update may supersede a queued data update. The newer URL includes
	// the data params, so the final navigation must still load data.
	pendingRequiresLoad = !options.shallow || (pendingUrl !== null && pendingRequiresLoad);
	pendingUrl = next;
	const runGeneration = generation;

	const task = queue
		.catch(() => {})
		.then(async () => {
			if (runGeneration !== generation) return;
			// A newer mutation superseded this URL. It was built from this one, so
			// it already includes this change.
			if (pendingUrl !== next) return;

			try {
				await goto(next, {
					reset: false,
					replace: options.replace,
					// Never let a shallow change suppress a pending data load.
					shallow: options.shallow && !pendingRequiresLoad
				});
			} catch (error) {
				if (!isAbort(error)) {
					console.error('Issue page navigation failed', error);
				}
			} finally {
				if (pendingUrl === next) {
					pendingUrl = null;
					pendingRequiresLoad = false;
				}
			}
		});

	queue = task;
	return task;
}

// Call when the user navigates by history or leaves the page so queued URL
// changes from the previous view cannot override the new location.
export function resetIssueUrlQueue(): void {
	generation += 1;
	pendingUrl = null;
	pendingRequiresLoad = false;
	queue = Promise.resolve();
}
