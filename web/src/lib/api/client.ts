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
	approval_level: string;
	description: string;
	instructions: string;
	place: string;
	purchase_date: string | null;
	purchase_price: string | null;
	expected_available_date: string | null;
	import_batch_id: string | null;
	manager_notes: string;
	current_booking_id: string | null;
	current_booking_status: string | null;
	current_booking_end_date: string | null;
	current_booking_unit_name: string | null;
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

export interface Unit {
	id: string;
	name: string;
	type: string;
}

export interface Booking {
	id: string;
	created_by: string;
	used_by_unit_id: string | null;
	used_by_external: string | null;
	used_by_external_contact: string | null;
	unit_name: string | null;
	status: string;
	start_date: string;
	end_date: string;
	notes: string;
	created_at: string;
}

export interface BookingItem {
	id: string;
	booking_id: string;
	article_id: string;
	commercial_name: string;
	common_name: string;
	location_name: string;
	category_name: string;
	place: string;
	article_status: string;
	article_expected_available_date: string | null;
	approval_level: string;
	individually_tracked: boolean;
	pickup_status: string | null;
	return_status: string | null;
}

export interface ArticleEvent {
	id: string;
	article_id: string;
	actor_id: string;
	actor_name: string;
	event_type: string;
	description: string;
	metadata: Record<string, string>;
	created_at: string;
}

export interface BookingEvent {
	id: string;
	booking_id: string;
	actor_id: string;
	actor_name: string;
	event_type: string;
	message: string;
	metadata: Record<string, any>;
	created_at: string;
}

export interface AvailabilityGroup {
	commercial_name: string;
	available_count: number;
	reported_usable_count: number;
	incoming_count: number;
	under_repair_count: number;
	approval_level: string;
	category_name: string;
	location_name: string;
}

export interface GroupSettings {
	notification_email_from: string;
	smtp_key_set: boolean;
	smtp_key_masked: string;
	gchat_webhook_url: string;
	default_approval_level: string;
}

interface FetchOptions {
	fetch?: typeof globalThis.fetch;
}

export class ApiError extends Error {
	statusCode: number;
	body: Record<string, any>;
	constructor(message: string, statusCode: number, body: Record<string, any>) {
		super(message);
		this.statusCode = statusCode;
		this.body = body;
	}
}

async function request<T>(path: string, opts: FetchOptions = {}): Promise<T> {
	const f = opts.fetch ?? globalThis.fetch;
	const res = await f(`${API_BASE}${path}`, { headers: { 'Content-Type': 'application/json' } });
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(body.error || res.statusText, res.status, body);
	}
	return res.json();
}

async function requestMut<T>(path: string, method: string, body: unknown, opts: FetchOptions = {}): Promise<T> {
	const f = opts.fetch ?? globalThis.fetch;
	const res = await f(`${API_BASE}${path}`, {
		method,
		headers: { 'Content-Type': 'application/json' },
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		const b = await res.json().catch(() => ({}));
		throw new ApiError(b.error || res.statusText, res.status, b);
	}
	if (res.status === 204) return undefined as T;
	return res.json();
}

export function createApiClient(opts: FetchOptions = {}) {
	return {
		listArticles: (params?: { search?: string; category_id?: string; location_id?: string; status?: string; mine?: boolean; with_availability?: boolean; date?: string }) => {
			const query = new URLSearchParams();
			if (params?.search) query.set('search', params.search);
			if (params?.category_id) query.set('category_id', params.category_id);
			if (params?.location_id) query.set('location_id', params.location_id);
			if (params?.status) query.set('status', params.status);
			if (params?.mine) query.set('mine', 'true');
			if (params?.with_availability) query.set('with_availability', 'true');
			if (params?.date) query.set('date', params.date);
			const qs = query.toString();
			return request<Article[]>(`/articles${qs ? '?' + qs : ''}`, opts);
		},
		listLocations: () => request<Location[]>('/locations', opts),
		listCategories: () => request<Category[]>('/categories', opts),
		checkAvailability: (startDate: string, endDate: string, params?: { category_id?: string; location_id?: string; bookable_only?: boolean }) => {
			const query = new URLSearchParams({ start_date: startDate, end_date: endDate });
			if (params?.category_id) query.set('category_id', params.category_id);
			if (params?.location_id) query.set('location_id', params.location_id);
			if (params?.bookable_only === false) query.set('bookable_only', 'false');
			return request<AvailabilityGroup[]>(`/articles/availability?${query}`, opts);
		},
		createBooking: (data: { start_date: string; end_date: string; notes?: string; used_by_unit_id?: string; used_by_external?: string }) =>
			requestMut<Booking>('/bookings', 'POST', data, opts),
		listBookings: () => request<Booking[]>('/bookings', opts),
		getBooking: (id: string) => request<{ booking: Booking; items: BookingItem[] }>(`/bookings/${id}`, opts),
		updateBooking: (id: string, data: Record<string, unknown>) =>
			requestMut<Booking>(`/bookings/${id}`, 'PUT', data, opts),
		addBookingItems: (bookingId: string, commercialName: string, quantity: number, locationName?: string) =>
			requestMut<BookingItem[]>(`/bookings/${bookingId}/items`, 'POST', { commercial_name: commercialName, quantity, location_name: locationName }, opts),
		removeBookingItem: (bookingId: string, itemId: string) =>
			requestMut<void>(`/bookings/${bookingId}/items/${itemId}`, 'DELETE', undefined, opts),
		cancelBooking: (id: string) =>
			requestMut<Booking | void>(`/bookings/${id}/cancel`, 'POST', {}, opts),
		copyBooking: (id: string) =>
			requestMut<{ booking: Booking; items_copied: number; items_total: number }>(`/bookings/${id}/copy`, 'POST', {}, opts),
		pickupBooking: (id: string) =>
			requestMut<Booking>(`/bookings/${id}/pickup`, 'POST', {}, opts),
		updateItemPickup: (bookingId: string, itemId: string, pickupStatus: string, articleStatus?: string, comment?: string) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/pickup`, 'PUT', {
				pickup_status: pickupStatus,
				...(articleStatus ? { article_status: articleStatus } : {}),
				...(comment ? { comment } : {})
			}, opts),
		swapItem: (bookingId: string, itemId: string, newArticleId: string) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/swap`, 'POST', { new_article_id: newArticleId }, opts),
		listAvailableArticles: (startDate: string, endDate: string, params?: { exclude_booking_id?: string; commercial_name?: string }) => {
			const query = new URLSearchParams({ start_date: startDate, end_date: endDate });
			if (params?.exclude_booking_id) query.set('exclude_booking_id', params.exclude_booking_id);
			if (params?.commercial_name) query.set('commercial_name', params.commercial_name);
			return request<{ id: string; commercial_name: string; common_name: string; location_name: string; place: string; status: string; expected_available_date: string | null }[]>(`/articles/availability/articles?${query}`, opts);
		},
		returnBooking: (id: string) =>
			requestMut<Booking>(`/bookings/${id}/return`, 'POST', {}, opts),
		updateItemReturn: (bookingId: string, itemId: string, data: { return_status: string; expected_return_date?: string; notes?: string }) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/return`, 'PUT', data, opts),
		listUnits: () => request<Unit[]>('/units', opts),

		// Approval
		approveBooking: (id: string, message?: string) =>
			requestMut<Booking>(`/bookings/${id}/approve`, 'POST', { message: message ?? '' }, opts),
		rejectBooking: (id: string, message?: string) =>
			requestMut<Booking>(`/bookings/${id}/reject`, 'POST', { message: message ?? '' }, opts),
		submitBooking: (id: string, message?: string, forceApproval?: boolean) =>
			requestMut<Booking>(`/bookings/${id}/submit`, 'POST', {
				...(message ? { message } : {}),
				...(forceApproval ? { force_approval: true } : {})
			}, opts),
		listBookingEvents: (bookingId: string) =>
			request<BookingEvent[]>(`/bookings/${bookingId}/events`, opts),
		addBookingNote: (bookingId: string, message: string) =>
			requestMut<BookingEvent>(`/bookings/${bookingId}/events`, 'POST', { message }, opts),
		listPendingApprovals: () => request<Booking[]>('/bookings?status=submitted', opts),

		// Article status & events
		updateArticleStatus: (articleId: string, data: { status: string; comment?: string }) =>
			requestMut<Article>(`/articles/${articleId}/status`, 'PUT', data, opts),
		listArticleEvents: (articleId: string, limit?: number) => {
			const query = new URLSearchParams();
			if (limit) query.set('limit', String(limit));
			const qs = query.toString();
			return request<{ events: ArticleEvent[]; has_more: boolean }>(`/articles/${articleId}/events${qs ? '?' + qs : ''}`, opts);
		},
		listArticleGroupEvents: (articleId: string, limit?: number) => {
			const query = new URLSearchParams();
			if (limit) query.set('limit', String(limit));
			const qs = query.toString();
			return request<{ events: ArticleEvent[]; has_more: boolean }>(`/articles/${articleId}/group-events${qs ? '?' + qs : ''}`, opts);
		},
		addArticleNote: (articleId: string, message: string) =>
			requestMut<void>(`/articles/${articleId}/events`, 'POST', { message }, opts),

		// Group settings
		getGroupSettings: () => request<GroupSettings>('/group-settings', opts),
		updateGroupSettings: (data: { notification_email_from?: string; smtp_key?: string | null; gchat_webhook_url?: string; default_approval_level?: string }) =>
			requestMut<GroupSettings>('/group-settings', 'PUT', data, opts),

		// Article CRUD
		getArticle: (id: string) => request<Article>(`/articles/${id}`, opts),
		createArticle: (data: Record<string, unknown>) =>
			requestMut<Article>('/articles', 'POST', data, opts),
		updateArticle: (id: string, data: Record<string, unknown>, group?: boolean) =>
			requestMut<Article>(`/articles/${id}${group ? '?group=true' : ''}`, 'PUT', data, opts),
		deleteArticle: (id: string) =>
			requestMut<void>(`/articles/${id}`, 'DELETE', undefined, opts),

		// Bulk operations
		bulkUpdateArticles: (data: { article_ids: string[]; status?: string; location_id?: string; approval_level?: string; comment?: string }) =>
			requestMut<{ updated: number; conflicts: Array<{ article_id: string; article_name: string; booking_id: string; booking_dates: string; booking_unit: string }> }>('/articles/bulk', 'PUT', data, opts),
		updateGroupCount: (data: { commercial_name: string; location_id: string; new_count: number }) =>
			requestMut<{ count: number }>('/articles/group-count', 'POST', data, opts),

		// CSV import
		importArticles: async (file: File, mode: 'preview' | 'confirmed' = 'preview', duplicateAction?: string) => {
			const f = opts.fetch ?? globalThis.fetch;
			const formData = new FormData();
			formData.append('file', file);
			if (duplicateAction) formData.append('duplicate_action', duplicateAction);
			const res = await f(`${API_BASE}/articles/import?mode=${mode}`, {
				method: 'POST',
				body: formData
			});
			if (!res.ok) {
				const b = await res.json().catch(() => ({}));
				throw new Error(b.error || res.statusText);
			}
			return res.json();
		},

		// Location CRUD
		createLocation: (data: { name: string; sort_order?: number }) =>
			requestMut<Location>('/locations', 'POST', data, opts),
		updateLocation: (id: string, data: { name: string; sort_order?: number }) =>
			requestMut<Location>(`/locations/${id}`, 'PUT', data, opts),
		deleteLocation: (id: string) =>
			requestMut<void>(`/locations/${id}`, 'DELETE', undefined, opts),

		// Category CRUD
		createCategory: (data: { name: string; sort_order?: number }) =>
			requestMut<Category>('/categories', 'POST', data, opts),
		updateCategory: (id: string, data: { name: string; sort_order?: number }) =>
			requestMut<Category>(`/categories/${id}`, 'PUT', data, opts),
		deleteCategory: (id: string) =>
			requestMut<void>(`/categories/${id}`, 'DELETE', undefined, opts),
	};
}
