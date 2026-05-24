import type { PageServerLoad } from './$types';

const DEV_MODE = process.env.DEV_MODE === 'true';
const DEMO_MODE = process.env.DEMO_MODE === 'true';
const DEMO_URL = process.env.DEMO_URL || 'https://demo.scoutrustning.se';
const PROD_URL = process.env.PROD_URL || 'https://scoutrustning.se';

export const load: PageServerLoad = async () => {
	return {
		demo: DEMO_MODE,
		dev: DEV_MODE,
		demoUrl: DEMO_URL,
		prodUrl: PROD_URL
	};
};
