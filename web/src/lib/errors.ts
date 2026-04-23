import { ApiError } from '$lib/api/client';
import * as m from '$lib/paraglide/messages.js';

type MsgFn = ((params?: Record<string, string>) => string) | undefined;
const messages = m as unknown as Record<string, MsgFn>;

// Maps API error keys to i18n message keys.
// Spaces/hyphens in the API key become underscores to match message key format.
function errorKey(apiKey: string): string {
	return 'error_' + apiKey.replace(/[^a-zA-Z0-9_]+/g, '_').replace(/_+/g, '_').replace(/^_|_$/g, '');
}

// Translates an API error (or any caught error) into a user-facing string.
// User-facing errors are mapped to i18n keys; internal/5xx errors show a generic message.
export function translateError(e: unknown): string {
	if (e instanceof ApiError) {
		if (e.statusCode >= 500) return messages.error_generic?.() ?? e.message;
		const key = errorKey(e.message);
		const fn = messages[key];
		if (fn) return fn(e.body?.params);
		return messages.error_generic?.() ?? e.message;
	}
	if (e instanceof Error) return messages.error_generic?.() ?? e.message;
	return messages.error_generic?.() ?? String(e);
}
