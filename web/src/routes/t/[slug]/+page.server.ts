import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getTool, getInstall } from '$lib/api';

export const load: PageServerLoad = async ({ fetch, params }) => {
	try {
		const [tool, install] = await Promise.all([
			getTool(fetch, params.slug),
			getInstall(fetch, params.slug)
		]);
		return { tool, install };
	} catch {
		throw error(404, 'Tool not found');
	}
};
