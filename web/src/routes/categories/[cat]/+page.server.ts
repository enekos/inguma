import type { PageServerLoad } from './$types';
import { getBrowse } from '$lib/api';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const tools = await getBrowse(fetch, { category: params.cat });
	return { category: params.cat, tools };
};
