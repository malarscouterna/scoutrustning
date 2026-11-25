import type { Booking } from '#lib/api/client.ts';

/**
 * UI status for a booking. A picked_up booking whose items are not all
 * collected is shown as "picking_up" ("Utlämning pågår") rather than claiming
 * everything has been fetched. This is derived from item progress - no new
 * database status - so promoting it to a real status later is a one-line change.
 *
 * `itemsPickedUp` is passed explicitly by the detail page (which has the items
 * loaded); list cards use Booking.all_picked_up from the list endpoint.
 */
export function displayBookingStatus(booking: Pick<Booking, 'status' | 'all_picked_up'>, itemsPickedUp = booking.all_picked_up): string {
	if (booking.status === 'picked_up' && itemsPickedUp === false) return 'picking_up';
	return booking.status;
}
