import type { PageServerLoad } from './$types';

const DEMO_MODE = process.env.DEMO_MODE === 'true';

export const load: PageServerLoad = async () => {
	return { demo: DEMO_MODE };
};
