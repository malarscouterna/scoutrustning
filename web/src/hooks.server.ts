import type { Handle } from '@sveltejs/kit';

const API_URL = process.env.API_URL || 'http://localhost:8080';

export const handle: Handle = async ({ event, resolve }) => {
	if (event.url.pathname.startsWith('/api/')) {
		const target = `${API_URL}${event.url.pathname}${event.url.search}`;
		const headers = new Headers(event.request.headers);

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
