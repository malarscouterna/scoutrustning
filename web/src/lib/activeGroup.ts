// Switches the active group by asking the server to set the httpOnly
// active-group-id cookie, then reloads so the new group's data loads.
// No page or component should write this cookie directly - see hooks.server.ts.
export async function switchGroup(groupId: string): Promise<void> {
	await fetch('/group/switch', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ groupId })
	});
	location.reload();
}
