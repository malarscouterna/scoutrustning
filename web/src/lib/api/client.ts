const API_BASE = '/api/v0';

export interface Article {
	id: string;
	commercial_name: string;
	common_name: string;
	category_id: string;
	category_name: string;
	location_id: string;
	location_name: string;
	status: string;
	individually_tracked: boolean;
	requires_approval: boolean;
	description: string;
	place: string;
}

export interface Location {
	id: string;
	name: string;
	sort_order: number;
}

export interface Category {
	id: string;
	name: string;
	parent_id: string | null;
	sort_order: number;
}

interface FetchOptions {
	fetch?: typeof globalThis.fetch;
	persona?: string;
}

async function request<T>(path: string, opts: FetchOptions = {}): Promise<T> {
	const f = opts.fetch ?? globalThis.fetch;
	const headers: Record<string, string> = { 'Content-Type': 'application/json' };
	if (opts.persona) {
		headers['X-Dev-Role-Override'] = opts.persona;
	}

	const res = await f(`${API_BASE}${path}`, { headers });
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.error || res.statusText);
	}
	return res.json();
}

export function createApiClient(opts: FetchOptions = {}) {
	return {
		listArticles: (params?: { search?: string; category_id?: string; location_id?: string; status?: string }) => {
			const query = new URLSearchParams();
			if (params?.search) query.set('search', params.search);
			if (params?.category_id) query.set('category_id', params.category_id);
			if (params?.location_id) query.set('location_id', params.location_id);
			if (params?.status) query.set('status', params.status);
			const qs = query.toString();
			return request<Article[]>(`/articles${qs ? '?' + qs : ''}`, opts);
		},
		listLocations: () => request<Location[]>('/locations', opts),
		listCategories: () => request<Category[]>('/categories', opts),
	};
}
