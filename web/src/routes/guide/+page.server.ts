import { readFileSync } from 'fs';
import { resolve } from 'path';
import { marked } from 'marked';
import type { PageServerLoad } from './$types';

const PROD_URL = process.env.PROD_URL || 'https://scoutrustning.se';

let cachedHtml: string | null = null;

export const load: PageServerLoad = async () => {
	if (!cachedHtml) {
		try {
			const md = readFileSync(resolve(process.cwd(), 'guide.md'), 'utf-8');
			cachedHtml = await marked(md);
		} catch {
			cachedHtml = '<p>Guiden kunde inte laddas.</p>';
		}
	}
	return { html: cachedHtml, prodUrl: PROD_URL };
};
