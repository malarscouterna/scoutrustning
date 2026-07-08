import type { PageServerLoad } from './$types';

const DEV_MODE = process.env.DEV_MODE === 'true';
const DEMO_MODE = process.env.DEMO_MODE === 'true';
const DEMO_URL = process.env.DEMO_URL || 'https://demo.scoutrustning.se';
const PROD_URL = process.env.PROD_URL || 'https://scoutrustning.se';

export const load: PageServerLoad = async ({ parent }) => {
	const { user } = await parent();
	return {
		user,
		demo: DEMO_MODE,
		// DEV_MODE alone is also true in demo (it just gates the persona switcher there,
		// behind a real login) - this flag drives the try-demo/try-prod link matrix below,
		// which needs "true local dev, not demo" specifically, or the try-demo link would
		// incorrectly show while already in demo.
		dev: DEV_MODE && !DEMO_MODE,
		demoUrl: DEMO_URL,
		prodUrl: PROD_URL
	};
};
