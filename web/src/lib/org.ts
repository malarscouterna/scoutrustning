import * as m from '#lib/paraglide/messages.js';

// ScoutID organisation types. Scoutrustning currently registers groups and
// districts, but the type list mirrors the full memberships claim.
export type OrgType =
	| 'group'
	| 'district'
	| 'region'
	| 'organisation'
	| 'corps'
	| 'network'
	| 'project';

// Types offered in the signup form. The backend accepts the full list, so a
// later rollout only needs to extend this array.
export const SIGNUP_ORG_TYPES: OrgType[] = ['group', 'district'];

export function orgTypeLabel(type: string | null | undefined): string {
	switch (type) {
		case 'group':
			return m.org_type_group();
		case 'district':
			return m.org_type_district();
		case 'region':
			return m.org_type_region();
		case 'network':
			return m.org_type_network();
		case 'project':
			return m.org_type_project();
		default:
			return m.org_type_organisation();
	}
}
