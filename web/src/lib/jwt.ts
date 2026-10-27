// Decodes a JWT's payload without verifying the signature. Verification
// already happened server-side (Auth.js session) or at the Go API boundary -
// this is only used to read display/prefill claims in SvelteKit load functions.
export function decodeJwtPayload(token: string): Record<string, any> | null {
	try {
		const base64Url = token.split('.')[1];
		if (!base64Url) return null;

		// Convert Base64URL to standard Base64
		const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
		
		// Add padding if necessary
		const padded = base64.padEnd(base64.length + (4 - (base64.length % 4)) % 4, '=');

		return JSON.parse(
			new TextDecoder().decode(Uint8Array.from(atob(padded), (c) => c.charCodeAt(0)))
		);
	} catch {
		return null;
	}
}
