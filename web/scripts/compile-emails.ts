// Generated HTML files are written to api/internal/notifications/templates/.
// Run: pnpm compile-emails
// Called automatically by pnpm build.
import mjml2html from 'mjml';
import { readFileSync, writeFileSync, mkdirSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const emailsDir = resolve(__dirname, '../src/lib/emails');
const outDir = resolve(__dirname, '../../api/internal/notifications/templates');

mkdirSync(outDir, { recursive: true });

const templates = ['booking', 'issue'];

for (const name of templates) {
	const mjmlSrc = readFileSync(`${emailsDir}/${name}.mjml`, 'utf-8');
	// Use 'soft' validation so placeholder strings (EMAIL_BANNER_BG etc.) pass
	// color attribute checks. Structural errors still cause non-zero exit.
	const { html, errors } = await mjml2html(mjmlSrc, { validationLevel: 'soft' });

	const structural = errors.filter((e: any) => e.severity === 'error');
	if (structural.length > 0) {
		console.error(`${name}.mjml errors:`);
		for (const e of structural) console.error(' ', e.formattedMessage);
		process.exit(1);
	}
	if (errors.length > 0) {
		for (const e of errors) console.warn(`  warn: ${e.formattedMessage}`);
	}

	const outPath = `${outDir}/${name}.html`;
	writeFileSync(outPath, `<!-- Generated from web/src/lib/emails/${name}.mjml - do not edit directly -->\n` + html);
	console.log(`compiled ${name}.mjml -> ${outPath}`);
}
