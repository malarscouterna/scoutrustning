export interface User {
	member_id: string;
	group_id: string;
	name: string;
	email: string;
	roles: string[];
	units: string[];
}

export function hasRole(user: User | null, role: string): boolean {
	return user?.roles.includes(role) ?? false;
}
