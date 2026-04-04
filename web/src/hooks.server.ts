import type { Handle } from '@sveltejs/kit';

const API_URL = process.env.API_URL || 'http://localhost:8080';
const DEV_MODE = process.env.DEV_MODE === 'true';
const PERSONA_COOKIE = 'dev-persona';
const DEFAULT_PERSONA = 'leader-yggdrasil';

export const handle: Handle = async ({ event, resolve }) => {
	// Dev mode: POST /dev/persona — set the active persona cookie
	if (DEV_MODE && event.url.pathname === '/dev/persona' && event.request.method === 'POST') {
		const { persona } = await event.request.json();
		const maxAge = 60 * 60 * 24 * 30;
		return new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				'Content-Type': 'application/json',
				'Set-Cookie': `${PERSONA_COOKIE}=${persona}; Path=/; Max-Age=${maxAge}; SameSite=Lax`
			}
		});
	}

	// Proxy /api/* to Go API
	if (event.url.pathname.startsWith('/api/')) {
		const target = `${API_URL}${event.url.pathname}${event.url.search}`;
		const headers = new Headers(event.request.headers);

		// Dev mode: inject persona header. Production: forward real auth (Phase 3).
		if (DEV_MODE) {
			const persona = event.cookies.get(PERSONA_COOKIE) || DEFAULT_PERSONA;
			headers.set('X-Dev-Role-Override', persona);
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

		return new Response(res.body, {
			status: res.status,
			statusText: res.statusText,
			headers: res.headers
		});
	}

	return resolve(event);
};
