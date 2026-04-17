import type { Handle } from '@sveltejs/kit';
import { redirect } from '@sveltejs/kit';
import { sequence } from '@sveltejs/kit/hooks';
import { authHandle } from './auth';

const API_URL = process.env.API_URL || 'http://localhost:8080';
const DEV_MODE = process.env.DEV_MODE === 'true';
const DEMO_MODE = process.env.DEMO_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';
const DEFAULT_PERSONA = 'leader-yggdrasil';

function isPublicPath(pathname: string): boolean {
	return pathname.startsWith('/auth/') || pathname === '/login';
}

function isTokenExpired(token: string): boolean {
	try {
		const payload = JSON.parse(atob(token.split('.')[1]));
		return !payload.exp || payload.exp * 1000 < Date.now();
	} catch {
		return true;
	}
}

function clearAuthCookies(event: any): void {
	event.cookies.delete('__Secure-authjs.session-token', { path: '/' });
	event.cookies.delete('__Secure-authjs.callback-url', { path: '/' });
	event.cookies.delete('authjs.session-token', { path: '/' });
}

async function getAccessToken(event: any): Promise<string | null> {
	try {
		const session = await event.locals.auth?.();
		const token = (session as any)?.accessToken ?? null;
		if (token && isTokenExpired(token)) return null;
		return token;
	} catch {
		return null;
	}
}

const appHandle: Handle = async ({ event, resolve }) => {
	// Dev mode: POST /dev/persona — set or clear the active persona cookie
	if (DEV_MODE && event.url.pathname === '/dev/persona' && event.request.method === 'POST') {
		const { persona } = await event.request.json();
		const maxAge = 60 * 60 * 24 * 30;
		const cookie = persona
			? `${PERSONA_COOKIE}=${persona}; Path=/; Max-Age=${maxAge}; SameSite=Lax`
			: `${PERSONA_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`;
		return new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				'Content-Type': 'application/json',
				'Set-Cookie': cookie
			}
		});
	}

	// Skip auth for public paths
	if (isPublicPath(event.url.pathname)) {
		return resolve(event);
	}

	// Determine auth state
	let authMode: 'persona' | 'oidc' | 'none' = 'none';
	let personaKey: string | undefined;
	let accessToken: string | null = null;

	if (DEV_MODE) {
		personaKey = event.cookies.get(PERSONA_COOKIE);
		if (personaKey) {
			if (DEMO_MODE) {
				// Demo: persona cookie only valid with an OIDC session
				accessToken = await getAccessToken(event);
				if (accessToken) {
					authMode = 'persona';
				} else {
					// Stale persona cookie without OIDC - clear it and redirect to login
					event.cookies.delete(PERSONA_COOKIE, { path: '/' });
					clearAuthCookies(event);
					const callbackUrl = event.url.pathname + event.url.search;
					throw redirect(302, `/login?callbackUrl=${encodeURIComponent(callbackUrl)}`);
				}
			} else {
				authMode = 'persona';
			}
		} else {
			accessToken = await getAccessToken(event);
			if (accessToken) {
				authMode = 'oidc';
			} else if (DEMO_MODE) {
				// Demo: require OIDC login, no auto-persona fallback
				const callbackUrl = event.url.pathname + event.url.search;
				throw redirect(302, `/login?callbackUrl=${encodeURIComponent(callbackUrl)}`);
			} else {
				// Dev fallback: set default persona
				event.cookies.set(PERSONA_COOKIE, DEFAULT_PERSONA, { path: '/', maxAge: 60 * 60 * 24 * 30 });
				personaKey = DEFAULT_PERSONA;
				authMode = 'persona';
			}
		}
	} else {
		accessToken = await getAccessToken(event);
		if (accessToken) {
			authMode = 'oidc';
		} else {
			// Production: redirect to Keycloak via Auth.js
			// Clear any stale Auth.js cookies before redirecting to prevent redirect loops
			// caused by an unreadable session cookie interfering with authHandle.
			clearAuthCookies(event);
			const callbackUrl = event.url.pathname + event.url.search;
			throw redirect(302, `/login?callbackUrl=${encodeURIComponent(callbackUrl)}`);
		}
	}

	// Proxy /api/* to Go API
	if (event.url.pathname.startsWith('/api/')) {
		const target = `${API_URL}${event.url.pathname}${event.url.search}`;
		const headers = new Headers(event.request.headers);

		// Strip auth headers from client — only the server decides identity
		headers.delete('Authorization');
		headers.delete('X-Dev-Role-Override');

		if (authMode === 'persona') {
			headers.set('X-Dev-Role-Override', personaKey!);
		} else if (authMode === 'oidc' && accessToken) {
			headers.set('Authorization', `Bearer ${accessToken}`);
		}

		const res = await fetch(target, {
			method: event.request.method,
			headers,
			body: event.request.method !== 'GET' && event.request.method !== 'HEAD'
				? event.request.body
				: undefined,
			// @ts-expect-error duplex needed for streaming body
			duplex: 'half'
		});

		// If the API says the group doesn't exist, the 403 response passes through.
		// The layout redirects unmapped users to / so page load functions won't hit this.
		return new Response(res.body, {
			status: res.status,
			statusText: res.statusText,
			headers: res.headers
		});
	}

	return resolve(event);
};

const hasOIDC = !!(process.env.AUTH_KEYCLOAK_ID && process.env.AUTH_KEYCLOAK_SECRET && process.env.AUTH_KEYCLOAK_ISSUER);

export const handle: Handle = hasOIDC
	? sequence(authHandle, appHandle)
	: appHandle;
