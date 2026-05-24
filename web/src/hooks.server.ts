import type { Handle } from '@sveltejs/kit';
import { redirect } from '@sveltejs/kit';
import { sequence } from '@sveltejs/kit/hooks';
import { authHandle } from './auth';
import { i18n } from '$lib/i18n';

const API_URL = process.env.API_URL || 'http://localhost:8080';
const DEV_MODE = process.env.DEV_MODE === 'true';
const DEMO_MODE = process.env.DEMO_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';
const DEFAULT_PERSONA = 'leader-yggdrasil';

function isPublicPath(pathname: string): boolean {
	return pathname.startsWith('/auth/') || pathname === '/login' || pathname === '/guide';
}

function isTokenExpired(token: string): boolean {
	try {
		const payload = JSON.parse(atob(token.split('.')[1]));
		return !payload.exp || payload.exp * 1000 < Date.now();
	} catch {
		return true;
	}
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
			// Production: redirect to login — stale cookie cleanup handled by the outer wrapper
			const callbackUrl = event.url.pathname + event.url.search;
			throw redirect(302, `/login?callbackUrl=${encodeURIComponent(callbackUrl)}`);
		}
	}

	// Proxy /api/* to Go API
	if (event.url.pathname.startsWith('/api/')) {
		const target = `${API_URL}${event.url.pathname}${event.url.search}`;
		const headers = new Headers(event.request.headers);

		// Strip auth and identity headers from client — only the server decides these
		headers.delete('Authorization');
		headers.delete('X-Dev-Role-Override');
		headers.delete('X-Language');

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

const innerHandle: Handle = hasOIDC
	? sequence(authHandle, i18n.handle(), appHandle)
	: sequence(i18n.handle(), appHandle);

// Outer wrapper: post-processes all responses, including thrown redirects.
// throw redirect() propagates as a JS exception and bypasses normal response
// processing — we catch it here so we can inspect and modify the response.
// If a redirect to /login happens while the browser still holds a stale
// Auth.js session cookie, we append deletion headers to break the loop.
export const handle: Handle = async ({ event, resolve }) => {
	let response: Response;
	try {
		response = await innerHandle({ event, resolve });
	} catch (thrown: unknown) {
		// SvelteKit's redirect() throws a Redirect object with status + location.
		// Convert it to a real Response so we can post-process it below.
		if (thrown && typeof thrown === 'object' && 'status' in thrown && 'location' in thrown) {
			const r = thrown as { status: number; location: string };
			response = new Response(null, { status: r.status, headers: { Location: r.location } });
		} else {
			throw thrown;
		}
	}

	const requestCookies = event.request.headers.get('cookie') ?? '';
	const location = response.headers.get('location') ?? '';
	const redirectingToLogin = response.status >= 300 && response.status < 400 && location.includes('/login');

	if (redirectingToLogin) {
		const hasSessionCookie = requestCookies.includes('authjs.session-token');
		console.log(`[auth] redirect to login — session cookie present: ${hasSessionCookie}, path: ${event.url.pathname}`);
		if (hasSessionCookie) {
			console.log('[auth] stale session cookie detected — clearing');
			const headers = new Headers(response.headers);
			headers.append('Set-Cookie', '__Secure-authjs.session-token=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax; Secure');
			headers.append('Set-Cookie', '__Secure-authjs.callback-url=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax; Secure');
			headers.append('Set-Cookie', 'authjs.session-token=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax');
			return new Response(null, { status: response.status, headers });
		}
	}

	return response;
};
