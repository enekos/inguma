import type { PageServerLoad } from './$types';
import { search } from '$lib/api';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const q = url.searchParams.get('q') ?? '';
	const kind = url.searchParams.get('kind') ?? '';
	const harness = url.searchParams.get('harness') ?? '';
	const category = url.searchParams.get('category') ?? '';
	const platform = url.searchParams.get('platform') ?? '';

	if (!q) return { q, kind, harness, category, platform, results: [], error: null };
	try {
		const results = await search(fetch, q, { kind, harness, category, platform });
		return { q, kind, harness, category, platform, results, error: null };
	} catch (e) {
		const msg = e instanceof Error ? e.message : 'search failed';
		return { q, kind, harness, category, platform, results: [], error: msg };
	}
};
