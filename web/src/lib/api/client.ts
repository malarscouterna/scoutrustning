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
	image_path: string | null;
	image_ids: string[];
	current_booking_id: string | null;
	current_booking_status: string | null;
	current_booking_end_date: string | null;
	current_booking_team_name: string | null;
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

export interface Team {
	id: string;
	name: string;
	type: string;
	access_level: string;
	claim_mappings: { claim_scope: string; claim_id: string }[];
}

export interface PerEventPrefs {
	gruppkanal?: boolean;
	personal_email_policy?: 'always' | 'if_no_broadcast' | 'never';
}

export interface TeamNotifSettings {
	notification_email: string;
	notification_prefs: Record<string, PerEventPrefs>;
	gchat_space_id: string;
	/** null = inherit group default; [] = explicit opt-out; [...] = explicit selection */
	gruppkanal_channels: string[] | null;
	/** Group-level default, returned alongside the team's own settings for display purposes. */
	default_gruppkanal_channels: string[];
}

export interface Booking {
	id: string;
	created_by: string;
	used_by_team_id: string | null;
	used_by_external: string | null;
	used_by_external_contact: string | null;
	team_name: string | null;
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
	location_id: string;
	location_name: string;
	category_name: string;
	place: string;
	article_description: string;
	article_instructions: string;
	article_status: string;
	article_expected_available_date: string | null;
	approval_level: string;
	individually_tracked: boolean;
	image_ids: string[];
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
	metadata: Record<string, any>;
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
	image_ids: string[];
	location_id: string;
	description: string;
	instructions: string;
}

export interface GroupSettings {
	notification_email_from: string;
	smtp_host: string;
	smtp_port: number;
	smtp_tls: string;
	smtp_user: string;
	smtp_key_set: boolean;
	smtp_key_masked: string;
	system_smtp_configured: boolean;
	system_smtp_from: string;
	gchat_configured: boolean;
	gchat_admin_email: string;
	default_approval_level: string;
	default_access_unknown: string;
	default_access_troop: string;
	default_access_role: string;
	image_upload_role: string;
	booking_role: string;
	article_edit_role: string;
	issue_resolve_role: string;
	manager_notes_role: string;
	default_language: string;
	notification_channels: string[];
}

export interface ResolvedPref {
	policy: 'always' | 'if_no_broadcast' | 'never';
	source: 'user' | 'team_default' | 'group_default' | 'system_default';
	default_policy: 'always' | 'if_no_broadcast' | 'never';
}

export type NotificationPrefs = Record<string, ResolvedPref>;

export interface IssueArticle {
	id: string;
	commercial_name: string;
	common_name: string;
	location_name: string;
	individually_tracked: boolean;
}

export interface IssueAssignee {
	user_id: string;
	user_name: string;
	assigned_at: string;
}

export interface GroupMember {
	id: string;
	name: string;
	email: string;
	access_level: string;
}

export interface IssueEvent {
	id: string;
	issue_id: string;
	actor_id: string;
	actor_name: string;
	event_type: string;
	description: string;
	metadata: Record<string, any>;
	created_at: string;
}

// Issue as returned by GET /issues (list)
export interface Issue {
	id: string;
	title: string;
	description: string;
	severity: 'usable' | 'unusable' | 'missing';
	status: 'open' | 'in_progress' | 'resolved' | 'archived';
	reporter_id: string;
	reporter_name: string;
	booking_id: string | null;
	created_at: string;
	updated_at: string;
	articles: IssueArticle[];
}

// Issue as returned by GET /issues/:id, POST /issues, etc. (detail)
export interface IssueDetail {
	id: string;
	title: string;
	description: string;
	severity: 'usable' | 'unusable' | 'missing';
	status: 'open' | 'in_progress' | 'resolved' | 'archived';
	reporter: { id: string; name: string };
	booking_id: string | null;
	created_at: string;
	updated_at: string;
	articles: IssueArticle[];
	assignees: IssueAssignee[];
	events: IssueEvent[];
}

export interface SharedImage {
	id: string;
	file_id: string;
	title: string;
	description: string;
	format: string;
	shared: boolean;
	attribution: string;
	created_at: string;
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
		getMe: () => request<{ member_id: string; group_id: string; group_name: string; name: string; email: string; teams: { team_id: string; team_name: string; team_type: string; access_level: string }[]; max_access: string }>('/me', opts),
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
			if (params?.bookable_only === true) query.set('bookable_only', 'true');
			return request<AvailabilityGroup[]>(`/articles/availability?${query}`, opts);
		},
		createBooking: (data: { start_date: string; end_date: string; notes?: string; used_by_team_id?: string; used_by_external?: string }) =>
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
		updateItemPickup: (bookingId: string, itemId: string, pickupStatus: string, articleStatus?: string, comment?: string, imageIds?: string[]) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/pickup`, 'PUT', {
				pickup_status: pickupStatus,
				...(articleStatus ? { article_status: articleStatus } : {}),
				...(comment ? { comment } : {}),
				...(imageIds?.length ? { image_ids: imageIds } : {})
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
		updateItemReturn: (bookingId: string, itemId: string, data: { return_status: string; expected_return_date?: string; notes?: string; image_ids?: string[] }) =>
			requestMut<BookingItem>(`/bookings/${bookingId}/items/${itemId}/return`, 'PUT', data, opts),
		listTeams: () => request<Team[]>('/teams', opts),
		createTeam: (data: { name: string; type: string; access_level?: string; claim_scope?: string; claim_id?: string }) =>
			requestMut<Team>('/teams', 'POST', data, opts),
		updateTeam: (id: string, data: { name?: string; type?: string; access_level?: string }) =>
			requestMut<Team>(`/teams/${id}`, 'PUT', data, opts),
		deleteTeam: (id: string) =>
			requestMut<void>(`/teams/${id}`, 'DELETE', undefined, opts),

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
		updateArticleStatus: (articleId: string, data: { status: string; comment?: string; image_ids?: string[] }) =>
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
		addArticleNote: (articleId: string, message: string, imageIds?: string[]) =>
			requestMut<void>(`/articles/${articleId}/events`, 'POST', { message, ...(imageIds?.length ? { image_ids: imageIds } : {}) }, opts),

		// Group settings
		getGroupSettings: () => request<GroupSettings>('/group-settings', opts),
		updateGroupSettings: (data: { notification_email_from?: string; smtp_host?: string; smtp_port?: number; smtp_tls?: string; smtp_user?: string; smtp_key?: string | null; default_approval_level?: string; default_language?: string }) =>
			requestMut<GroupSettings>('/group-settings', 'PUT', data, opts),
		uploadGchatKey: (keyJson: string) =>
			requestMut<{ gchat_configured: boolean; gchat_admin_email: string; spaces: { name: string; displayName: string }[] }>('/group-settings/gchat-key', 'POST', JSON.parse(keyJson), opts),
		deleteGchatKey: () =>
			requestMut<void>('/group-settings/gchat-key', 'DELETE', undefined, opts),
		listGchatSpaces: () =>
			request<{ name: string; displayName: string }[]>('/group-settings/gchat-spaces', opts),
		setTeamGchatSpace: (teamId: string, gchatSpaceId: string) =>
			requestMut<void>(`/teams/${teamId}/gchat-space`, 'PUT', { gchat_space_id: gchatSpaceId }, opts),
		clearTeamGchatSpace: (teamId: string) =>
			requestMut<void>(`/teams/${teamId}/gchat-space`, 'DELETE', undefined, opts),
		updateLanguage: (language: string | null) =>
			requestMut<void>('/me/language', 'PUT', { language }, opts),
		getNotificationPrefs: () =>
			request<{ prefs: NotificationPrefs }>('/me/notification-prefs', opts),
		updateNotificationPrefs: (data: Record<string, PerEventPrefs | null>) =>
			requestMut<void>('/me/notification-prefs', 'PUT', data, opts),
		resetNotificationPrefs: () =>
			requestMut<void>('/me/notification-prefs', 'DELETE', undefined, opts),
		sendTestEmail: () =>
			requestMut<{ sent?: boolean; skipped?: boolean }>('/me/test-email', 'POST', undefined, opts),
		getGroupNotificationDefaults: () =>
			request<{
				defaults: Record<string, PerEventPrefs>;
				system_defaults: Record<string, PerEventPrefs>;
				default_gruppkanal_channels: string[];
			}>('/group-settings/notification-defaults', opts),
		updateGroupNotificationDefaults: (data: { defaults: Record<string, PerEventPrefs>; default_gruppkanal_channels: string[] }) =>
			requestMut<void>('/group-settings/notification-defaults', 'PUT', data, opts),
		forceGroupNotificationDefaults: () =>
			requestMut<{ reset_user_count: number; reset_team_count: number }>('/group-settings/force-notification-defaults', 'POST', undefined, opts),
		getTeamNotificationSettings: (id: string) =>
			request<TeamNotifSettings>(`/teams/${id}/notification-settings`, opts),
		updateTeamNotificationSettings: (id: string, data: Partial<Pick<TeamNotifSettings, 'notification_email' | 'notification_prefs' | 'gruppkanal_channels'>>) =>
			requestMut<void>(`/teams/${id}/notification-settings`, 'PUT', data, opts),
		updateTeamName: (id: string, name: string) =>
			requestMut<Team>(`/teams/${id}/name`, 'PUT', { name }, opts),

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
			requestMut<{ updated: number; conflicts: Array<{ article_id: string; article_name: string; booking_id: string; booking_dates: string; booking_team: string }> }>('/articles/bulk', 'PUT', data, opts),
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

		uploadIssueImage: async (file: File) => {
			const f = opts.fetch ?? globalThis.fetch;
			const formData = new FormData();
			formData.append('file', file);
			const res = await f(`${API_BASE}/images/issue`, {
				method: 'POST',
				body: formData
			});
			if (!res.ok) {
				const b = await res.json().catch(() => ({}));
				throw new ApiError(b.error || res.statusText, res.status, b);
			}
			return res.json() as Promise<{ image_id: string }>;
		},

		uploadProductImage: async (file: Blob | File, commercialName: string, locationId: string, meta?: { title?: string; description?: string; format?: string; shared?: boolean; attribution?: string }) => {
			const f = opts.fetch ?? globalThis.fetch;
			const formData = new FormData();
			formData.append('file', file, file instanceof File ? file.name : 'crop.jpg');
			formData.append('commercial_name', commercialName);
			formData.append('location_id', locationId);
			if (meta?.title) formData.append('title', meta.title);
			if (meta?.description) formData.append('description', meta.description);
			if (meta?.format) formData.append('format', meta.format);
			if (meta?.shared) formData.append('shared', 'true');
			if (meta?.attribution) formData.append('attribution', meta.attribution);
			const res = await f(`${API_BASE}/images/product`, {
				method: 'POST',
				body: formData
			});
			if (!res.ok) {
				const b = await res.json().catch(() => ({}));
				throw new ApiError(b.error || res.statusText, res.status, b);
			}
			return res.json() as Promise<{ image: Record<string, any>; image_ids: string[] }>;
		},
		deleteProductImage: async (imageId: string, commercialName: string, locationId: string) => {
			const query = new URLSearchParams({ commercial_name: commercialName, location_id: locationId });
			return requestMut<void>(`/images/product/${imageId}?${query}`, 'DELETE', undefined, opts);
		},
		getProductImageMeta: (imageId: string) =>
			request<{ id: string; file_id: string; title: string; description: string; format: string; shared: boolean; attribution: string; ref_count: number }>(`/images/product/${imageId}`, opts),
		updateProductImage: (imageId: string, data: { title: string; description: string; shared: boolean; attribution: string }) =>
			requestMut<{ id: string; file_id: string; title: string; description: string; format: string; shared: boolean }>(`/images/product/${imageId}`, 'PUT', data, opts),
		listMyImages: () =>
			request<{ id: string; file_id: string; title: string; description: string; format: string; shared: boolean; created_at: string; own_group_count: number; other_group_count: number }[]>('/images/my', opts),
		listArticlesUsingImage: (imageId: string) =>
			request<{ commercial_name: string; location_name: string; article_id: string }[]>(`/images/my/${imageId}/articles`, opts),
		deleteMyImage: (imageId: string) =>
			requestMut<void>(`/images/my/${imageId}`, 'DELETE', undefined, opts),
		listProductImages: (commercialName: string, locationId: string) => {
			const query = new URLSearchParams({ commercial_name: commercialName, location_id: locationId });
			return request<{ id: string; file_id: string; title: string; description: string; format: string; shared: boolean; attribution: string; uploaded_by: string; created_at: string }[]>(`/images/product?${query}`, opts);
		},
		listSharedImages: (search?: string) => {
			const query = new URLSearchParams();
			if (search) query.set('search', search);
			const qs = query.toString();
			return request<SharedImage[]>(`/images/shared${qs ? '?' + qs : ''}`, opts);
		},
		addFromShared: (sourceImageId: string, commercialName: string, locationId: string, title: string, description: string) =>
			requestMut<{ image: Record<string, any>; image_ids: string[] }>('/images/product/from-shared', 'POST', {
				source_image_id: sourceImageId,
				commercial_name: commercialName,
				location_id: locationId,
				title,
				description
			}, opts),
		reorderProductImages: (commercialName: string, locationId: string, imageIds: string[]) =>
			requestMut<{ image_ids: string[] }>('/images/product/reorder', 'PUT', {
				commercial_name: commercialName,
				location_id: locationId,
				image_ids: imageIds
			}, opts),

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

		// Issues
		listIssues: (params?: { status?: string; mine?: boolean; article_id?: string }) => {
			const query = new URLSearchParams();
			if (params?.status) query.set('status', params.status);
			if (params?.mine) query.set('mine', 'true');
			if (params?.article_id) query.set('article_id', params.article_id);
			const qs = query.toString();
			return request<Issue[]>(`/issues${qs ? '?' + qs : ''}`, opts);
		},
		createIssue: (data: { article_id: string; severity: string; description: string; booking_id?: string; image_ids?: string[]; count?: number }) =>
			requestMut<IssueDetail>('/issues', 'POST', data, opts),
		getIssue: (id: string) =>
			request<IssueDetail>(`/issues/${id}`, opts),
		updateIssue: (id: string, data: { title?: string; description?: string; status?: string; comment?: string }) =>
			requestMut<IssueDetail>(`/issues/${id}`, 'PUT', data, opts),
		addIssueComment: (id: string, data: { description: string; image_ids?: string[] }) =>
			requestMut<IssueDetail>(`/issues/${id}/comments`, 'POST', data, opts),
		replaceIssueAssignees: (id: string, userIds: string[]) =>
			requestMut<IssueDetail>(`/issues/${id}/assignees`, 'PUT', { user_ids: userIds }, opts),
		addIssueAssignee: (id: string, userId: string) =>
			requestMut<void>(`/issues/${id}/assignees`, 'POST', { user_id: userId }, opts),
		removeIssueAssignee: (id: string, userId: string) =>
			requestMut<void>(`/issues/${id}/assignees/${userId}`, 'DELETE', undefined, opts),
		listGroupMembers: (accessLevels?: string) =>
			request<GroupMember[]>(`/users${accessLevels ? `?access_levels=${accessLevels}` : ''}`, opts),
		addIssueArticle: (id: string, articleId: string) =>
			requestMut<IssueDetail>(`/issues/${id}/articles`, 'POST', { article_id: articleId }, opts),
		removeIssueArticle: (id: string, articleId: string) =>
			requestMut<void>(`/issues/${id}/articles/${articleId}`, 'DELETE', undefined, opts),
	};
}
