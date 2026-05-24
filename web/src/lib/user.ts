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
	notification_email: string | null;
	language: string;
	teams: TeamMembership[];
	max_access: 'view' | 'book' | 'trusted' | 'manager';
	permissions?: {
		image_upload: string;
		booking: string;
		article_edit: string;
		issue_resolve: string;
		manager_notes: string;
	};
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
	const required = user?.permissions?.booking ?? 'book';
	return accessAtLeast(user?.max_access, required);
}

export function canEditArticles(user: User | null): boolean {
	const required = user?.permissions?.article_edit ?? 'manager';
	return accessAtLeast(user?.max_access, required);
}

export function canResolveIssues(user: User | null): boolean {
	const required = user?.permissions?.issue_resolve ?? 'manager';
	return accessAtLeast(user?.max_access, required);
}

export function canSeeManagerNotes(user: User | null): boolean {
	const required = user?.permissions?.manager_notes ?? 'manager';
	return accessAtLeast(user?.max_access, required);
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
