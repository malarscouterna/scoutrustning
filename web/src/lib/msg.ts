import * as m from '$lib/paraglide/messages.js';

const messages = m as unknown as Record<string, (() => string) | undefined>;

export function msg(key: string): string | undefined {
	return messages[key]?.();
}
