import { redirect } from '@sveltejs/kit';
import { decodeJwtPayload } from '#lib/jwt.ts';
import { SIGNUP_ORG_TYPES } from '#lib/org.ts';
import type { PageServerLoad } from './$types';

const DEV_MODE = process.env.DEV_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';

interface RoleOption {
	id: number;
	key: string;
	name: string;
}

interface OrgOption {
	id: string;
	type: string;
	name: string;
	roles: RoleOption[];
	isPrimary: boolean;
}

// Dev mode without an OIDC session has no memberships claim to read from. Show
// representative organisations so the signup UI can still be exercised. These
// mirror the tenants provisioned by dev-seed.sh; submission is not wired up.
const DEV_MOCK_ORGS: OrgOption[] = [
	{
		id: '766',
		type: 'group',
		name: 'Mälarscouterna',
		roles: [{ id: 1, key: 'material_responsible', name: 'Utrustningsansvarig' }],
		isPrimary: true
	},
	{
		id: '999',
		type: 'group',
		name: 'Testkåren',
		roles: [{ id: 2, key: 'admin', name: 'Admin' }],
		isPrimary: false
	},
	{
		id: '111',
		type: 'district',
		name: 'Mälardalens distrikt',
		roles: [{ id: 68, key: 'distriktsstyrelsen', name: 'Distriktsstyrelsen' }],
		isPrimary: false
	}
];

// Entity types in the ScoutID memberships claim, in the order we want them to
// appear in the signup dropdown. Only types listed in SIGNUP_ORG_TYPES are
// exposed; the backend accepts the full list for tenants provisioned later.
const ORG_SOURCES = [
	{ key: 'groups', type: 'group' },
	{ key: 'districts', type: 'district' },
	{ key: 'regions', type: 'region' },
	{ key: 'organisations', type: 'organisation' },
	{ key: 'corps', type: 'corps' },
	{ key: 'networks', type: 'network' },
	{ key: 'projects', type: 'project' }
] as const;

const SIGNUP_TYPES = new Set<string>(SIGNUP_ORG_TYPES);
const SIGNUP_SOURCES = ORG_SOURCES.filter((source) => SIGNUP_TYPES.has(source.type));

export const load: PageServerLoad = async ({ locals, url, cookies, parent }) => {
	const session = (await locals.auth?.()) as any;

	// Dev mode: a persona cookie counts as "logged in" even without a real OIDC
	// session. There's no JWT to read memberships from in that case, so show
	// mock organisations rather than rendering the form with no selectors.
	if (!session?.accessToken || session?.error) {
		if (DEV_MODE && cookies.get(PERSONA_COOKIE)) {
			const { user } = await parent();
			return { name: user?.name ?? '', email: user?.email ?? '', orgs: DEV_MOCK_ORGS };
		}
		throw redirect(302, `/welcome?callbackUrl=${encodeURIComponent(url.pathname)}`);
	}

	const payload = decodeJwtPayload(session.accessToken);
	const name = payload?.name ?? '';
	const email = payload?.email ?? '';

	const orgs: OrgOption[] = [];
	const memberships = payload?.memberships ?? {};
	for (const { key, type } of SIGNUP_SOURCES) {
		const entries = memberships?.[key] ?? {};
		for (const [id, org] of Object.entries<any>(entries)) {
			orgs.push({
				id,
				type,
				name: org?.name ?? '',
				// Higher-level entities expose only a role key, never a name.
				roles: (org?.roles ?? []).map((r: any) => ({
					id: r.id,
					key: r.key,
					name: r.name || r.key
				})),
				isPrimary: !!org?.is_primary
			});
		}
	}

	const typeOrder = new Map<string, number>(SIGNUP_SOURCES.map((source, i) => [source.type, i]));
	orgs.sort((a, b) => {
		if (a.isPrimary !== b.isPrimary) return Number(b.isPrimary) - Number(a.isPrimary);
		const aOrder = typeOrder.get(a.type) ?? 99;
		const bOrder = typeOrder.get(b.type) ?? 99;
		if (aOrder !== bOrder) return aOrder - bOrder;
		return (a.name || a.id).localeCompare(b.name || b.id);
	});

	return { name, email, orgs };
};
