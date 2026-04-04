import { readFileSync, existsSync } from 'fs';
import { resolve } from 'path';
import type { LayoutServerLoad } from './$types';
import type { User } from '$lib/user';

const DEV_MODE = process.env.DEV_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';
const DEFAULT_PERSONA = 'leader-yggdrasil';

const ROLE_MAPPING_PATHS = [
	resolve(process.cwd(), 'role-mapping.json'),
	resolve(process.cwd(), '..', 'role-mapping.json'),
];

const PERSONAS_PATHS = [
	resolve(process.cwd(), 'dev-personas.json'),      // Docker mount: /app/dev-personas.json
	resolve(process.cwd(), '..', 'dev-personas.json'), // Local dev: ../dev-personas.json
];

let cachedPersonas: Record<string, User> | null = null;
let cachedRoleMapping: any = null;

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

function loadRoleMapping(): any {
	if (cachedRoleMapping) return cachedRoleMapping;
	for (const p of ROLE_MAPPING_PATHS) {
		if (existsSync(p)) {
			try {
				cachedRoleMapping = JSON.parse(readFileSync(p, 'utf-8'));
				return cachedRoleMapping;
			} catch { /* try next */ }
		}
	}
	return null;
}

function parseUserFromSession(session: any): User | null {
	const accessToken = session?.accessToken;
	if (!accessToken) return null;

	// Decode JWT payload (no verification — the Go API does that)
	try {
		const payload = JSON.parse(atob(accessToken.split('.')[1]));
		const rm = loadRoleMapping();
		if (!rm) return null;

		// Extract member ID from preferred_username ("scoutnet|3169207" → "3169207")
		let memberID = payload.preferred_username || '';
		const pipeIdx = memberID.indexOf('|');
		if (pipeIdx >= 0) memberID = memberID.substring(pipeIdx + 1);

		// Parse roles
		const tokenRoles: string[] = payload.roles || [];
		let groupID = '';
		const appRoles = new Set<string>();
		const units = new Set<string>();
		const roleUnits: Record<string, Set<string>> = {};

		for (const role of tokenRoles) {
			const parts = role.split(':');
			if (parts.length !== 3) continue;
			const [scope, id, roleName] = parts;

			if (scope === 'group') {
				if (!groupID) groupID = id;
				const gm = rm.groups?.[id];
				if (!gm) continue;
				if (gm.admin_roles?.[roleName]) {
					appRoles.add('equipment_manager');
					units.add(gm.admin_roles[roleName]);
					(roleUnits['equipment_manager'] ??= new Set()).add(gm.admin_roles[roleName]);
				}
				if (gm.project_roles?.[roleName]) {
					appRoles.add('project_leader');
					units.add(gm.project_roles[roleName]);
					(roleUnits['project_leader'] ??= new Set()).add(gm.project_roles[roleName]);
				}
			} else if (scope === 'troop') {
				appRoles.add('leader');
				for (const gm of Object.values(rm.groups || {}) as any[]) {
					if (gm.troops?.[id]) {
						units.add(gm.troops[id]);
						(roleUnits['leader'] ??= new Set()).add(gm.troops[id]);
						break;
					}
				}
			}
		}

		if (!groupID) return null;

		return {
			member_id: memberID,
			group_id: groupID,
			name: payload.name || '',
			email: payload.email || '',
			roles: [...appRoles],
			units: [...units],
			role_units: Object.fromEntries(
				Object.entries(roleUnits).map(([k, v]) => [k, [...v]])
			),
		};
	} catch {
		return null;
	}
}

export const load: LayoutServerLoad = async ({ cookies, locals }) => {
	// Dev mode with persona cookie: use persona
	if (DEV_MODE) {
		const personaCookie = cookies.get(PERSONA_COOKIE);
		const personas = loadPersonas();

		if (personaCookie && personas[personaCookie]) {
			return {
				user: personas[personaCookie],
				dev: { personas, currentPersona: personaCookie }
			};
		}

		// No persona cookie — try OIDC session
		const session = await locals.auth?.();
		const user = parseUserFromSession(session);
		if (user) {
			return { user, dev: { personas, currentPersona: null } };
		}

		// Fallback to default persona in dev
		return {
			user: personas[DEFAULT_PERSONA] ?? null,
			dev: { personas, currentPersona: DEFAULT_PERSONA }
		};
	}

	// Production: user from OIDC session (hooks.server.ts handles redirect if no session)
	const session = await locals.auth?.();
	const user = parseUserFromSession(session);
	return { user, dev: null };
};
