import { readFileSync, existsSync } from 'fs';
import { resolve } from 'path';
import type { LayoutServerLoad } from './$types';
import type { User } from '$lib/user';

const DEV_MODE = process.env.DEV_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';
const DEFAULT_PERSONA = 'leader-yggdrasil';

const PERSONAS_PATHS = [
	resolve(process.cwd(), 'dev-personas.json'),      // Docker mount: /app/dev-personas.json
	resolve(process.cwd(), '..', 'dev-personas.json'), // Local dev: ../dev-personas.json
];

let cachedPersonas: Record<string, User> | null = null;

function loadPersonas(): Record<string, User> {
	if (cachedPersonas) return cachedPersonas;
	for (const p of PERSONAS_PATHS) {
		if (existsSync(p)) {
			try {
				const data = JSON.parse(readFileSync(p, 'utf-8'));
				cachedPersonas = data.personas;
				return cachedPersonas!;
			} catch { /* try next */ }
		}
	}
	return {};
}

export const load: LayoutServerLoad = async ({ cookies }) => {
	if (!DEV_MODE) {
		// Production: user will come from OIDC session (Phase 3)
		return { user: null, dev: null };
	}

	const personas = loadPersonas();
	const currentPersona = cookies.get(PERSONA_COOKIE) || DEFAULT_PERSONA;
	const user: User | null = personas[currentPersona] ?? null;

	return {
		user,
		dev: { personas, currentPersona }
	};
};
