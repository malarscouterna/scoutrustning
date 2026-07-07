import { redirect } from '@sveltejs/kit';
import { decodeJwtPayload } from '$lib/jwt';
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
	name: string;
	roles: RoleOption[];
	isPrimary: boolean;
}

export const load: PageServerLoad = async ({ locals, url, cookies, parent }) => {
	const session = (await locals.auth?.()) as any;

	// Dev mode: a persona cookie counts as "logged in" even without a real OIDC
	// session. There's no JWT to read memberships from in that case, so the
	// applicant just fills the org fields in manually.
	if (!session?.accessToken || session?.error) {
		if (DEV_MODE && cookies.get(PERSONA_COOKIE)) {
			const { user } = await parent();
			return { name: user?.name ?? '', email: user?.email ?? '', orgs: [] as OrgOption[] };
		}
		throw redirect(302, `/welcome?callbackUrl=${encodeURIComponent(url.pathname)}`);
	}

	const payload = decodeJwtPayload(session.accessToken);
	const name = payload?.name ?? '';
	const email = payload?.email ?? '';

	const orgs: OrgOption[] = [];
	const groups = payload?.memberships?.groups ?? {};
	for (const [id, gm] of Object.entries<any>(groups)) {
		orgs.push({
			id,
			name: gm?.name ?? '',
			roles: (gm?.roles ?? [])
				.filter((r: any) => r?.name)
				.map((r: any) => ({ id: r.id, key: r.key, name: r.name })),
			isPrimary: !!gm?.is_primary
		});
	}
	orgs.sort((a, b) => Number(b.isPrimary) - Number(a.isPrimary));

	return { name, email, orgs };
};
