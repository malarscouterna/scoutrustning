import { readFileSync, existsSync } from 'fs';
import { resolve } from 'path';
import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import type { User } from '$lib/user';

const DEV_MODE = process.env.DEV_MODE === 'true';
const DEMO_MODE = process.env.DEMO_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';
const DEFAULT_PERSONA = 'leader-yggdrasil';

const PERSONAS_PATHS = [
	resolve(process.cwd(), 'dev-personas.json'),
	resolve(process.cwd(), '..', 'dev-personas.json'),
];

interface DevPersona {
	member_id: string;
	name: string;
	email: string;
	groups: Record<string, string[]>;
}

let cachedPersonas: Record<string, DevPersona> | null = null;

function loadPersonas(): Record<string, DevPersona> {
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

// Fetch the resolved user from the Go API via /api/v0/me.
// The API resolves teams + access levels from the DB.
async function fetchMe(fetchFn: typeof globalThis.fetch): Promise<User | null> {
	try {
		const res = await fetchFn('/api/v0/me');
		if (!res.ok) return null;
		return await res.json();
	} catch {
		return null;
	}
}

export const load: LayoutServerLoad = async ({ cookies, locals, url, fetch: skFetch }) => {
	if (DEV_MODE) {
		const personaCookie = cookies.get(PERSONA_COOKIE);
		const personas = loadPersonas();

		if (personaCookie && personas[personaCookie]) {
			if (DEMO_MODE) {
				// Demo: persona cookie only valid with an OIDC session
				const session = await locals.auth?.() as any;
				if (!session?.accessToken) {
					cookies.delete(PERSONA_COOKIE, { path: '/' });
					return { user: null, dev: null, demo: true, oidcName: null };
				}
			}
			// Persona selected - fetch resolved user from API
			const user = await fetchMe(skFetch);
			return {
				user,
				dev: { personas, currentPersona: personaCookie },
				demo: DEMO_MODE,
				oidcName: null
			};
		}

		// No persona cookie — try OIDC session
		const session = await locals.auth?.() as any;
		if (session?.accessToken) {
			const user = await fetchMe(skFetch);
			if (user) {
				return { user, dev: { personas, currentPersona: null }, demo: DEMO_MODE, oidcName: null };
			}
			// Logged in via OIDC but group not found — show friendly message
			const oidcName = extractNameFromToken(session.accessToken);
			if (oidcName) {
				if (url.pathname !== '/') throw redirect(302, '/');
				return { user: null, dev: { personas, currentPersona: null }, demo: DEMO_MODE, oidcName };
			}
		}

		if (DEMO_MODE) {
			return { user: null, dev: null, demo: true, oidcName: null };
		}

		// Dev fallback to default persona — set cookie so hooks proxy sends it
		cookies.set(PERSONA_COOKIE, DEFAULT_PERSONA, { path: '/', httpOnly: false, secure: false, sameSite: 'lax' });
		const user = await fetchMe(skFetch);
		return {
			user,
			dev: { personas, currentPersona: DEFAULT_PERSONA },
			demo: false,
			oidcName: null
		};
	}

	// Production
	const session = await locals.auth?.() as any;
	if (!session?.accessToken) {
		return { user: null, dev: null, demo: false, oidcName: null };
	}
	const user = await fetchMe(skFetch);
	if (!user) {
		const oidcName = extractNameFromToken(session.accessToken);
		if (oidcName && url.pathname !== '/') throw redirect(302, '/');
		return { user: null, dev: null, demo: false, oidcName: oidcName ?? null };
	}
	return { user, dev: null, demo: false, oidcName: null };
};

function extractNameFromToken(token: string): string | null {
	try {
		const payload = JSON.parse(atob(token.split('.')[1]));
		return payload.name || null;
	} catch {
		return null;
	}
}
