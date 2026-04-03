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

export interface Unit {
	id: string;
	name: string;
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
	requires_approval: boolean;
	individually_tracked: boolean;
	pickup_status: string | null;
	return_status: string | null;
}

export interface AvailabilityGroup {
	commercial_name: string;
	available_count: number;
	requires_approval: boolean;
	category_name: string;
	location_name: string;
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

async function requestMut<T>(path: string, method: string, body: unknown, opts: FetchOptions = {}): Promise<T> {
	const f = opts.fetch ?? globalThis.fetch;
	const headers: Record<string, string> = { 'Content-Type': 'application/json' };
	if (opts.persona) {
		headers['X-Dev-Role-Override'] = opts.persona;
	}

	const res = await f(`${API_BASE}${path}`, {
		method,
		headers,
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		const b = await res.json().catch(() => ({}));
		throw new Error(b.error || res.statusText);
	}
	if (res.status === 204) return undefined as T;
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
		submitBooking: (id: string) =>
			requestMut<Booking>(`/bookings/${id}/submit`, 'POST', {}, opts),
		cancelBooking: (id: string) =>
			requestMut<Booking | void>(`/bookings/${id}/cancel`, 'POST', {}, opts),
		copyBooking: (id: string) =>
			requestMut<{ booking: Booking; items_copied: number; items_total: number }>(`/bookings/${id}/copy`, 'POST', {}, opts),
		pickupBooking: (id: string) =>
			requestMut<Booking>(`/bookings/${id}/pickup`, 'POST', {}, opts),
		updateItemPickup: (bookingId: string, itemId: string, pickupStatus: string) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/pickup`, 'PUT', { pickup_status: pickupStatus }, opts),
		swapItem: (bookingId: string, itemId: string, newArticleId: string) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/swap`, 'POST', { new_article_id: newArticleId }, opts),
		listAvailableArticles: (startDate: string, endDate: string, params?: { exclude_booking_id?: string; commercial_name?: string }) => {
			const query = new URLSearchParams({ start_date: startDate, end_date: endDate });
			if (params?.exclude_booking_id) query.set('exclude_booking_id', params.exclude_booking_id);
			if (params?.commercial_name) query.set('commercial_name', params.commercial_name);
			return request<{ id: string; commercial_name: string; common_name: string; location_name: string; place: string }[]>(`/articles/availability/articles?${query}`, opts);
		},
		returnBooking: (id: string) =>
			requestMut<Booking>(`/bookings/${id}/return`, 'POST', {}, opts),
		updateItemReturn: (bookingId: string, itemId: string, data: { return_status: string; expected_return_date?: string; notes?: string }) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/return`, 'PUT', data, opts),
		listUnits: () => request<Unit[]>('/units', opts),
	};
}
