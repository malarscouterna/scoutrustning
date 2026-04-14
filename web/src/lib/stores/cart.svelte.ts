import { browser } from '$app/environment';

const STORAGE_KEY = 'active-booking-id';

let activeBookingId = $state<string | null>(browser ? localStorage.getItem(STORAGE_KEY) : null);
let refreshVersion = $state(0);

export const cart = {
	get id() {
		return activeBookingId;
	},
	get active() {
		return activeBookingId !== null;
	},
	get refreshSignal() {
		return refreshVersion;
	},
	activate(bookingId: string) {
		activeBookingId = bookingId;
		if (browser) localStorage.setItem(STORAGE_KEY, bookingId);
	},
	clear() {
		activeBookingId = null;
		if (browser) localStorage.removeItem(STORAGE_KEY);
	},
	refresh() {
		refreshVersion++;
	}
};
