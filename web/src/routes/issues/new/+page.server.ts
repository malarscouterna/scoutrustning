import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url }) => {
	const articleId = url.searchParams.get('article_id') ?? '';
	const severity = url.searchParams.get('severity') ?? 'unusable';
	const bookingId = url.searchParams.get('booking_id') ?? '';
	return { prefill: { articleId, severity, bookingId } };
};
