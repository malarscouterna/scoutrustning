import { SvelteKitAuth } from '@auth/sveltekit';
import Keycloak from '@auth/sveltekit/providers/keycloak';

async function refreshAccessToken(token: any) {
	try {
		const issuer = process.env.AUTH_KEYCLOAK_ISSUER!;
		const res = await fetch(`${issuer}/protocol/openid-connect/token`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
			body: new URLSearchParams({
				grant_type: 'refresh_token',
				client_id: process.env.AUTH_KEYCLOAK_ID!,
				client_secret: process.env.AUTH_KEYCLOAK_SECRET!,
				refresh_token: token.refreshToken,
			}),
		});
		const tokens = await res.json();
		if (!res.ok) throw tokens;
		return {
			...token,
			accessToken: tokens.access_token,
			refreshToken: tokens.refresh_token ?? token.refreshToken,
			accessTokenExpires: Date.now() + (tokens.expires_in * 1000),
			error: undefined,
		};
	} catch (err) {
		console.error('[auth] token refresh failed:', err);
		return { ...token, error: 'RefreshAccessTokenError' };
	}
}

export const { handle: authHandle, signIn, signOut } = SvelteKitAuth({
	providers: [
		Keycloak({
			clientId: process.env.AUTH_KEYCLOAK_ID!,
			clientSecret: process.env.AUTH_KEYCLOAK_SECRET!,
			issuer: process.env.AUTH_KEYCLOAK_ISSUER!
		})
	],
	secret: process.env.AUTH_SECRET,
	trustHost: true,
	callbacks: {
		async jwt({ token, account }) {
			if (account) {
				return {
					...token,
					accessToken: account.access_token,
					refreshToken: account.refresh_token,
					accessTokenExpires: account.expires_at
						? account.expires_at * 1000
						: Date.now() + ((account.expires_in as number ?? 300) * 1000),
				};
			}
			// Returning null tells Auth.js to destroy the session, clearing all its
			// cookies (including chunked tokens) via its own signout mechanism.
			if (token.error === 'RefreshAccessTokenError') return null;
			if (!token.refreshToken) return null;
			if (Date.now() < (token.accessTokenExpires as number) - 30_000) return token;
			return refreshAccessToken(token);
		},
		async session({ session, token }) {
			(session as any).accessToken = token.accessToken;
			(session as any).error = token.error;
			return session;
		}
	}
});
