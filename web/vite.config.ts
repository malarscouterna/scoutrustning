import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { paraglideVitePlugin } from '@inlang/paraglide-js';
import { defineConfig } from 'vite';
import { cpSync } from 'node:fs';
import { resolve } from 'node:path';

export default defineConfig({
	plugins: [
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			// No URL-based language prefixing - language is resolved from the
			// paraglide_lang cookie (set server-side from the user's stored
			// language preference), never from the URL.
			strategy: ['cookie', 'baseLocale'],
			cookieName: 'paraglide_lang'
		}),
		tailwindcss(),
		sveltekit(),
		{
			name: 'copy-scout-components',
			buildStart() {
				cpSync(
					resolve('./node_modules/@scouterna/ui-webc/dist/ui-webc'),
					resolve('./static/ui-webc'),
					{ recursive: true }
				);
			}
		}
	]
});
