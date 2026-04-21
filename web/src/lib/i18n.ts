import { createI18n } from '@inlang/paraglide-sveltekit';
import * as runtime from '$lib/paraglide/runtime.js';

// exclude: all paths - disables URL-based language prefixing entirely.
// Language is resolved from the paraglide_lang cookie (set by +layout.server.ts)
// then Accept-Language header, then the source language ('sv').
export const i18n = createI18n(runtime, { exclude: [/\/.*/] });
