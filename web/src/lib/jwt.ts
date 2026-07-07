// Decodes a JWT's payload without verifying the signature. Verification
// already happened server-side (Auth.js session) or at the Go API boundary -
// this is only used to read display/prefill claims in SvelteKit load functions.
export function decodeJwtPayload(token: string): Record<string, any> | null {
	try {
		return JSON.parse(
			new TextDecoder().decode(Uint8Array.from(atob(token.split('.')[1]), (c) => c.charCodeAt(0)))
		);
	} catch {
		return null;
	}
}
