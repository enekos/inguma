import type { PageServerLoad } from './$types';
import { getBrowse, getCategories } from '$lib/api';

export const load: PageServerLoad = async ({ fetch }) => {
  const [tools, categories] = await Promise.all([
    getBrowse(fetch),
    getCategories(fetch)
  ]);
  return { tools, categories };
};
