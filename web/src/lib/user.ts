export interface TeamMembership {
	team_id: string;
	team_name: string;
	team_type: 'troop' | 'role';
	access_level: 'view' | 'book' | 'trusted' | 'manager';
}

export interface User {
	member_id: string;
	group_id: string;
	group_name: string;
	name: string;
	email: string;
	teams: TeamMembership[];
	max_access: 'view' | 'book' | 'trusted' | 'manager';
}

const accessOrder: Record<string, number> = {
	view: 0,
	book: 1,
	trusted: 2,
	manager: 3
};

export function accessAtLeast(level: string | undefined, required: string): boolean {
	return (accessOrder[level ?? 'view'] ?? 0) >= (accessOrder[required] ?? 0);
}

export function isManager(user: User | null): boolean {
	return user?.max_access === 'manager';
}

export function canBook(user: User | null): boolean {
	return accessAtLeast(user?.max_access, 'book');
}

// Backward compatibility shim — maps old role names to access checks.
// TODO: Remove once all call sites are migrated to isManager/canBook.
export function hasRole(user: User | null, role: string): boolean {
	switch (role) {
		case 'equipment_manager':
			return isManager(user);
		case 'project_leader':
			return accessAtLeast(user?.max_access, 'trusted');
		case 'leader':
			return accessAtLeast(user?.max_access, 'book');
		default:
			return false;
	}
}
