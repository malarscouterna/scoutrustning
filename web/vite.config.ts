import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { paraglide } from '@inlang/paraglide-sveltekit/vite';
import { defineConfig } from 'vite';
import { cpSync } from 'node:fs';
import { resolve } from 'node:path';

export default defineConfig({
	plugins: [
		paraglide({ project: './project.inlang', outdir: './src/lib/paraglide' }),
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
