import { SvelteKitAuth } from '@auth/sveltekit';
import Keycloak from '@auth/sveltekit/providers/keycloak';

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
			// On initial sign-in, persist the access token
			if (account) {
				token.accessToken = account.access_token;
			}
			return token;
		},
		async session({ session, token }) {
			// Expose access token to server-side session
			(session as any).accessToken = token.accessToken;
			return session;
		}
	}
});
