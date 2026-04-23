// Thin typed wrapper around apid. Every call uses the `fetch` passed in by
// SvelteKit load functions so SSR works and cookies/auth-headers are forwarded.

import { env } from '$env/dynamic/private';
import type { CategoryCount, IndexEntry, InstallResponse, SearchHit, ToolResponse } from './types';

export type Fetch = typeof fetch;

const DEFAULT_BASE = 'http://localhost:8091';

function baseURL(): string {
	// In production the frontend is proxied by Caddy so /api/* is same-origin.
	// In dev we fall back to localhost:8091 (where `bin/apid` runs).
	if (typeof window !== 'undefined') return '';
	return env.AGENTPOP_API_URL ?? DEFAULT_BASE;
}

async function getJSON<T>(fetchFn: Fetch, path: string): Promise<T> {
	const url = baseURL() + path;
	const res = await fetchFn(url);
	if (!res.ok) throw new Error(`${path} → ${res.status}`);
	return (await res.json()) as T;
}

export function getTool(fetchFn: Fetch, slug: string): Promise<ToolResponse> {
	return getJSON(fetchFn, `/api/tools/${encodeURIComponent(slug)}`);
}

export async function getInstall(fetchFn: Fetch, slug: string): Promise<InstallResponse> {
	return getJSON(fetchFn, `/api/install/${encodeURIComponent(slug)}`);
}

export async function getBrowse(
	fetchFn: Fetch,
	params: { category?: string; kind?: string; harness?: string; platform?: string } = {}
): Promise<IndexEntry[]> {
	const q = new URLSearchParams();
	for (const [k, v] of Object.entries(params)) if (v) q.set(k, v);
	const { tools } = await getJSON<{ tools: IndexEntry[] }>(
		fetchFn,
		`/api/tools${q.toString() ? '?' + q : ''}`
	);
	return tools;
}

export async function getCategories(fetchFn: Fetch): Promise<CategoryCount[]> {
	const { categories } = await getJSON<{ categories: CategoryCount[] }>(fetchFn, '/api/categories');
	return categories;
}

export async function search(
	fetchFn: Fetch,
	q: string,
	filters: { kind?: string; harness?: string; category?: string; platform?: string } = {}
): Promise<SearchHit[]> {
	const params = new URLSearchParams({ q });
	for (const [k, v] of Object.entries(filters)) if (v) params.set(k, v);
	const { results } = await getJSON<{ results: SearchHit[] }>(fetchFn, `/api/search?${params}`);
	return results;
}
