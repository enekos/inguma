# Agentpop Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the SvelteKit marketplace website: home page with search bar + categories + featured rows, search page with facet sidebar, tool detail page with per-harness install tabs, category listing, and static docs shell. SSR enabled so tool pages render fast and are crawlable.

**Architecture:** SvelteKit (Svelte 5) with SSR. Tailwind for styling, dark mode from day one. A tiny TypeScript `apiClient` module wraps `fetch` to the `apid` backend. Server-side `load` functions call the API, so initial navigations render on the server. All component state is derived from URL query params (search + filters) so results are shareable.

**Tech Stack:** Node 22, SvelteKit (latest), TypeScript, Tailwind CSS v4, Playwright for smoke tests, Vite (built into SvelteKit).

**Design spec:** `docs/superpowers/specs/2026-04-22-agentpop-marketplace-design.md`
**Depends on:** plans 1–3 merged (the frontend talks to `apid`).

---

## Prerequisites

- Node 22 (verify: `node --version` → v22.x).
- `bin/apid` runnable against `internal/api/testdata/corpus` for local dev.

All commands run from the repo root unless noted. The SvelteKit app lives under `web/`.

---

## File Structure

```
web/
├── package.json
├── pnpm-lock.yaml        # or package-lock.json (npm is fine)
├── svelte.config.js
├── vite.config.ts
├── tsconfig.json
├── tailwind.config.js
├── postcss.config.js
├── .gitignore
├── playwright.config.ts
├── src/
│   ├── app.html
│   ├── app.css           # tailwind entrypoint + global vars
│   ├── lib/
│   │   ├── api.ts        # typed API client
│   │   ├── types.ts      # shared types (Manifest, IndexEntry, Snippet, ...)
│   │   └── components/
│   │       ├── Nav.svelte
│   │       ├── SearchBar.svelte
│   │       ├── ToolCard.svelte
│   │       ├── FacetSidebar.svelte
│   │       └── InstallTabs.svelte
│   └── routes/
│       ├── +layout.svelte
│       ├── +layout.ts
│       ├── +page.svelte                # /
│       ├── +page.server.ts
│       ├── search/
│       │   ├── +page.svelte
│       │   └── +page.server.ts
│       ├── t/[slug]/
│       │   ├── +page.svelte
│       │   └── +page.server.ts
│       ├── categories/[cat]/
│       │   ├── +page.svelte
│       │   └── +page.server.ts
│       └── docs/
│           └── +page.svelte
├── tests/
│   └── smoke.spec.ts
└── static/
    └── favicon.svg
```

---

## Task 1: Scaffold SvelteKit + Tailwind

**Files:** bulk-created by scaffolder.

- [ ] **Step 1: Scaffold the project**

From the repo root, run:
```bash
mkdir -p web
cd web
npx sv@latest create . --template minimal --types ts --no-add-ons
```

(If `sv` prompts interactively for project name, pass `.` or press Enter to use the current dir. Select TypeScript, no formatting/linting add-ons.)

Note: The scaffolder produces a `.gitignore`, `package.json`, `svelte.config.js`, `vite.config.ts`, `tsconfig.json`, `src/app.html`, `src/app.d.ts`, a placeholder `src/routes/+page.svelte`, and a `static/` dir.

- [ ] **Step 2: Install Tailwind v4 and related deps**

```bash
cd web
npm install -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 3: Wire Tailwind into Vite**

Edit `web/vite.config.ts` so it contains:
```ts
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()]
});
```

- [ ] **Step 4: Create the Tailwind entrypoint**

Create `web/src/app.css`:
```css
@import 'tailwindcss';

/* Design tokens — switched by the .dark class on <html>. */
:root {
  color-scheme: light dark;
}
html { font-family: system-ui, -apple-system, sans-serif; }
```

- [ ] **Step 5: Import the stylesheet and add a dark-mode toggle**

Edit `web/src/routes/+layout.svelte` (create if the scaffolder didn't) to:
```svelte
<script lang="ts">
  import '../app.css';
  let { children } = $props();
</script>

<div class="min-h-screen bg-white text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
  {@render children()}
</div>
```

Edit `web/src/app.html` — inside `<head>`, add before `%sveltekit.head%`:
```html
<script>
  // Honour OS preference on first load; a future button can toggle .dark on <html>.
  if (matchMedia('(prefers-color-scheme: dark)').matches) {
    document.documentElement.classList.add('dark');
  }
</script>
```

- [ ] **Step 6: Verify dev server boots**

```bash
cd web
npm run dev -- --port 5173 &
DEV=$!
sleep 4
curl -s localhost:5173 | head -c 200
kill $DEV
```

Expected: HTML with a non-empty body (just the placeholder page is fine).

- [ ] **Step 7: Ensure the repo `.gitignore` covers `web/node_modules`**

Open repo-root `.gitignore` and add if missing:
```
/web/node_modules/
/web/.svelte-kit/
/web/build/
```

- [ ] **Step 8: Commit**

```bash
cd /Users/enekosarasola/agentpop
git add web .gitignore
git commit -m "chore(web): scaffold SvelteKit + Tailwind"
```

---

## Task 2: Typed API client

**Files:**
- Create: `web/src/lib/types.ts`
- Create: `web/src/lib/api.ts`

- [ ] **Step 1: Define shared types**

Create `web/src/lib/types.ts`:
```ts
// Shared types that mirror the apid JSON contract.
// Kept deliberately narrow — only the fields the UI reads.

export type Kind = 'mcp' | 'cli';

export interface IndexEntry {
  slug: string;
  display_name: string;
  description: string;
  kind: Kind;
  categories?: string[];
  tags?: string[];
  harnesses?: string[];
  platforms?: string[];
}

export interface Manifest {
  name: string;
  display_name: string;
  description: string;
  readme: string;
  license: string;
  kind: Kind;
  homepage?: string;
  categories?: string[];
  tags?: string[];
  compatibility: { harnesses: string[]; platforms: string[] };
  mcp?: {
    transport: 'stdio' | 'http';
    command?: string;
    args?: string[];
    url?: string;
    env?: { name: string; required: boolean; description?: string }[];
  };
  cli?: {
    bin: string;
    install: { type: 'npm' | 'go' | 'binary'; package?: string; module?: string; url_template?: string }[];
  };
}

export interface ToolResponse {
  slug: string;
  manifest: Manifest;
  readme: string;
}

export interface SearchHit {
  slug: string;
  score: number;
  tool: IndexEntry;
}

export interface Snippet {
  harness_id: string;
  display_name: string;
  format: 'json' | 'toml' | 'yaml' | 'shell';
  path?: string;
  content: string;
}

export interface InstallResponse {
  slug: string;
  cli: { command: string };
  snippets: Snippet[];
}

export interface CategoryCount {
  name: string;
  count: number;
}
```

- [ ] **Step 2: Write the client**

Create `web/src/lib/api.ts`:
```ts
// Thin typed wrapper around apid. Every call uses the `fetch` passed in by
// SvelteKit load functions so SSR works and cookies/auth-headers are forwarded.

import type {
  CategoryCount,
  IndexEntry,
  InstallResponse,
  SearchHit,
  ToolResponse
} from './types';

export type Fetch = typeof fetch;

const DEFAULT_BASE = 'http://localhost:8091';

function baseURL(): string {
  // In production the frontend is proxied by Caddy so /api/* is same-origin.
  // In dev we fall back to localhost:8091 (where `bin/apid` runs).
  if (typeof window !== 'undefined') return '';
  return process.env.AGENTPOP_API_URL ?? DEFAULT_BASE;
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

export async function getInstall(
  fetchFn: Fetch,
  slug: string
): Promise<InstallResponse> {
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
  const { categories } = await getJSON<{ categories: CategoryCount[] }>(
    fetchFn,
    '/api/categories'
  );
  return categories;
}

export async function search(
  fetchFn: Fetch,
  q: string,
  filters: { kind?: string; harness?: string; category?: string; platform?: string } = {}
): Promise<SearchHit[]> {
  const params = new URLSearchParams({ q });
  for (const [k, v] of Object.entries(filters)) if (v) params.set(k, v);
  const { results } = await getJSON<{ results: SearchHit[] }>(
    fetchFn,
    `/api/search?${params}`
  );
  return results;
}
```

- [ ] **Step 3: Commit**

```bash
cd /Users/enekosarasola/agentpop
git add web/src/lib
git commit -m "feat(web): typed API client"
```

---

## Task 3: Shared components (Nav, SearchBar, ToolCard)

**Files:**
- Create: `web/src/lib/components/Nav.svelte`
- Create: `web/src/lib/components/SearchBar.svelte`
- Create: `web/src/lib/components/ToolCard.svelte`

- [ ] **Step 1: Nav**

Create `web/src/lib/components/Nav.svelte`:
```svelte
<script lang="ts">
  // Top navigation visible on every page.
</script>

<nav class="border-b border-neutral-200 dark:border-neutral-800">
  <div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
    <a href="/" class="text-lg font-semibold">agentpop</a>
    <div class="flex gap-4 text-sm text-neutral-600 dark:text-neutral-400">
      <a href="/search" class="hover:underline">Search</a>
      <a href="/docs" class="hover:underline">Docs</a>
      <a href="https://github.com" class="hover:underline">GitHub</a>
    </div>
  </div>
</nav>
```

- [ ] **Step 2: SearchBar**

Create `web/src/lib/components/SearchBar.svelte`:
```svelte
<script lang="ts">
  interface Props { value?: string; autofocus?: boolean; }
  let { value = '', autofocus = false }: Props = $props();
</script>

<form action="/search" method="GET" class="flex gap-2">
  <input
    type="search"
    name="q"
    {value}
    {autofocus}
    placeholder="Search MCP servers and CLI tools…"
    class="w-full rounded-md border border-neutral-300 bg-white px-4 py-2 text-base
           focus:border-neutral-500 focus:outline-none
           dark:border-neutral-700 dark:bg-neutral-900"
  />
  <button
    type="submit"
    class="rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white
           dark:bg-neutral-100 dark:text-neutral-900"
  >
    Search
  </button>
</form>
```

- [ ] **Step 3: ToolCard**

Create `web/src/lib/components/ToolCard.svelte`:
```svelte
<script lang="ts">
  import type { IndexEntry } from '$lib/types';
  interface Props { tool: IndexEntry; }
  let { tool }: Props = $props();
</script>

<a
  href="/t/{tool.slug}"
  class="block rounded-lg border border-neutral-200 p-4 transition
         hover:border-neutral-400 hover:shadow-sm
         dark:border-neutral-800 dark:hover:border-neutral-600"
>
  <div class="flex items-start justify-between gap-3">
    <div class="min-w-0">
      <h3 class="truncate font-medium">{tool.display_name}</h3>
      <p class="mt-1 line-clamp-2 text-sm text-neutral-600 dark:text-neutral-400">
        {tool.description}
      </p>
    </div>
    <span
      class="shrink-0 rounded border border-neutral-300 px-2 py-0.5 text-xs
             uppercase tracking-wide text-neutral-600
             dark:border-neutral-700 dark:text-neutral-400"
    >
      {tool.kind}
    </span>
  </div>
  {#if tool.categories?.length}
    <div class="mt-3 flex flex-wrap gap-1.5">
      {#each tool.categories as c}
        <span class="rounded-full bg-neutral-100 px-2 py-0.5 text-xs text-neutral-700 dark:bg-neutral-800 dark:text-neutral-300">{c}</span>
      {/each}
    </div>
  {/if}
</a>
```

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/components
git commit -m "feat(web): Nav, SearchBar, ToolCard components"
```

---

## Task 4: Home page

**Files:**
- Modify: `web/src/routes/+page.svelte`
- Create: `web/src/routes/+page.server.ts`
- Modify: `web/src/routes/+layout.svelte` (add Nav)

- [ ] **Step 1: Load home data on the server**

Create `web/src/routes/+page.server.ts`:
```ts
import type { PageServerLoad } from './$types';
import { getBrowse, getCategories } from '$lib/api';

export const load: PageServerLoad = async ({ fetch }) => {
  const [tools, categories] = await Promise.all([
    getBrowse(fetch),
    getCategories(fetch)
  ]);
  return { tools, categories };
};
```

- [ ] **Step 2: Render the split home**

Replace `web/src/routes/+page.svelte` contents with:
```svelte
<script lang="ts">
  import type { PageData } from './$types';
  import SearchBar from '$lib/components/SearchBar.svelte';
  import ToolCard from '$lib/components/ToolCard.svelte';
  let { data }: { data: PageData } = $props();
  const featured = data.tools.slice(0, 6);
  const recent = data.tools.slice(0, 9);
</script>

<section class="mx-auto max-w-6xl px-6 py-16">
  <h1 class="text-4xl font-semibold tracking-tight">Find agentic tools.</h1>
  <p class="mt-2 text-lg text-neutral-600 dark:text-neutral-400">
    MCP servers and CLI tools for Claude Code, Cursor, and more.
  </p>
  <div class="mt-8 max-w-2xl">
    <SearchBar autofocus />
  </div>
</section>

<section class="mx-auto max-w-6xl px-6 pb-10">
  <h2 class="mb-4 text-sm font-semibold uppercase tracking-wide text-neutral-500">Categories</h2>
  <div class="flex flex-wrap gap-2">
    {#each data.categories as c}
      <a
        href="/categories/{c.name}"
        class="rounded-full border border-neutral-200 px-3 py-1 text-sm hover:border-neutral-400 dark:border-neutral-800 dark:hover:border-neutral-600"
      >
        {c.name}
        <span class="ml-1 text-xs text-neutral-500">{c.count}</span>
      </a>
    {/each}
  </div>
</section>

<section class="mx-auto max-w-6xl px-6 pb-10">
  <h2 class="mb-4 text-sm font-semibold uppercase tracking-wide text-neutral-500">Featured</h2>
  <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
    {#each featured as t}
      <ToolCard tool={t} />
    {/each}
  </div>
</section>

<section class="mx-auto max-w-6xl px-6 pb-16">
  <h2 class="mb-4 text-sm font-semibold uppercase tracking-wide text-neutral-500">Recently added</h2>
  <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
    {#each recent as t}
      <ToolCard tool={t} />
    {/each}
  </div>
</section>
```

- [ ] **Step 3: Wire Nav into the layout**

Edit `web/src/routes/+layout.svelte`:
```svelte
<script lang="ts">
  import '../app.css';
  import Nav from '$lib/components/Nav.svelte';
  let { children } = $props();
</script>

<div class="min-h-screen bg-white text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
  <Nav />
  {@render children()}
</div>
```

- [ ] **Step 4: Smoke the page against apid**

In terminal 1 (from repo root):
```bash
bin/apid -addr :8091 -corpus internal/api/testdata/corpus -marrow http://127.0.0.1:1 &
APID=$!
sleep 1
cd web && AGENTPOP_API_URL=http://localhost:8091 npm run dev -- --port 5173 &
DEV=$!
sleep 5
curl -s localhost:5173 | grep -c "Tool A"
kill $DEV $APID
```

Expected: count ≥ 1 (home lists the fixture tools).

- [ ] **Step 5: Commit**

```bash
cd /Users/enekosarasola/agentpop
git add web/src/routes web/src/routes/+page.server.ts
git commit -m "feat(web): home page with search + categories + featured"
```

---

## Task 5: Tool detail page + InstallTabs

**Files:**
- Create: `web/src/routes/t/[slug]/+page.server.ts`
- Create: `web/src/routes/t/[slug]/+page.svelte`
- Create: `web/src/lib/components/InstallTabs.svelte`

- [ ] **Step 1: Load data**

Create `web/src/routes/t/[slug]/+page.server.ts`:
```ts
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
  } catch (e) {
    throw error(404, 'Tool not found');
  }
};
```

- [ ] **Step 2: Build InstallTabs**

Create `web/src/lib/components/InstallTabs.svelte`:
```svelte
<script lang="ts">
  import type { InstallResponse } from '$lib/types';
  interface Props { install: InstallResponse; }
  let { install }: Props = $props();

  // "CLI one-liner" is its own virtual tab, always first.
  const tabs = [
    { id: '__cli__', label: 'CLI' },
    ...install.snippets.map((s) => ({ id: s.harness_id, label: s.display_name }))
  ];
  let active = $state(tabs[0].id);

  const activeSnippet = $derived(install.snippets.find((s) => s.harness_id === active));
</script>

<div class="rounded-lg border border-neutral-200 dark:border-neutral-800">
  <div class="flex border-b border-neutral-200 text-sm dark:border-neutral-800">
    {#each tabs as t}
      <button
        type="button"
        class={'px-4 py-2 ' + (active === t.id ? 'font-semibold' : 'text-neutral-500')}
        onclick={() => (active = t.id)}
      >
        {t.label}
      </button>
    {/each}
  </div>

  <div class="p-4">
    {#if active === '__cli__'}
      <p class="mb-2 text-sm text-neutral-600 dark:text-neutral-400">Run this in a terminal:</p>
      <pre class="overflow-x-auto rounded bg-neutral-900 p-3 text-sm text-neutral-100"><code
          >{install.cli.command}</code
        ></pre>
    {:else if activeSnippet}
      {#if activeSnippet.path}
        <p class="mb-2 text-sm text-neutral-600 dark:text-neutral-400">
          Paste into <code>{activeSnippet.path}</code>:
        </p>
      {/if}
      <pre class="overflow-x-auto rounded bg-neutral-900 p-3 text-sm text-neutral-100"><code
          >{activeSnippet.content}</code
        ></pre>
    {/if}
  </div>
</div>
```

- [ ] **Step 3: Render the detail page**

Create `web/src/routes/t/[slug]/+page.svelte`:
```svelte
<script lang="ts">
  import type { PageData } from './$types';
  import InstallTabs from '$lib/components/InstallTabs.svelte';
  let { data }: { data: PageData } = $props();
  const m = data.tool.manifest;
  // Strip the YAML frontmatter from index.md before rendering — the HTML is
  // kept minimal in v1; proper markdown rendering is a fast-follow.
  const readmeBody = data.tool.readme.replace(/^---[\s\S]*?---\s*/, '');
</script>

<article class="mx-auto max-w-4xl px-6 py-10">
  <header class="flex items-start justify-between gap-6">
    <div>
      <h1 class="text-3xl font-semibold">{m.display_name}</h1>
      <p class="mt-1 text-neutral-600 dark:text-neutral-400">{m.description}</p>
      <p class="mt-2 text-xs text-neutral-500">
        {m.license}
        {#if m.homepage}· <a href={m.homepage} class="underline">homepage ↗</a>{/if}
      </p>
    </div>
    <span
      class="rounded border border-neutral-300 px-2 py-0.5 text-xs uppercase tracking-wide
             text-neutral-600 dark:border-neutral-700 dark:text-neutral-400"
    >
      {m.kind}
    </span>
  </header>

  <section class="mt-8">
    <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-neutral-500">Install</h2>
    <InstallTabs install={data.install} />
  </section>

  <section class="mt-8">
    <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-neutral-500">README</h2>
    <pre class="whitespace-pre-wrap text-sm">{readmeBody}</pre>
  </section>

  <section class="mt-8 text-sm text-neutral-500">
    <div>Compatibility: {m.compatibility.harnesses.join(', ')}</div>
    <div>Platforms: {m.compatibility.platforms.join(', ')}</div>
  </section>
</article>
```

- [ ] **Step 4: Smoke**

With apid + dev server running (same way as Task 4 step 4):
```bash
curl -s localhost:5173/t/tool-a | grep -c "Demo readme body"
```

Expect ≥ 1.

- [ ] **Step 5: Commit**

```bash
git add web/src/routes/t web/src/lib/components/InstallTabs.svelte
git commit -m "feat(web): tool detail page with InstallTabs"
```

---

## Task 6: Search page + FacetSidebar

**Files:**
- Create: `web/src/routes/search/+page.server.ts`
- Create: `web/src/routes/search/+page.svelte`
- Create: `web/src/lib/components/FacetSidebar.svelte`

- [ ] **Step 1: Load**

Create `web/src/routes/search/+page.server.ts`:
```ts
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
```

- [ ] **Step 2: FacetSidebar**

Create `web/src/lib/components/FacetSidebar.svelte`:
```svelte
<script lang="ts">
  interface Props {
    q: string;
    kind: string;
    harness: string;
    category: string;
    platform: string;
  }
  let { q, kind, harness, category, platform }: Props = $props();
</script>

<form action="/search" method="GET" class="space-y-6 text-sm">
  <input type="hidden" name="q" value={q} />

  <fieldset>
    <legend class="mb-2 font-semibold">Kind</legend>
    {#each ['', 'mcp', 'cli'] as k}
      <label class="flex items-center gap-2">
        <input type="radio" name="kind" value={k} checked={kind === k} />
        <span class="capitalize">{k || 'any'}</span>
      </label>
    {/each}
  </fieldset>

  <fieldset>
    <legend class="mb-2 font-semibold">Harness</legend>
    {#each ['', 'claude-code', 'cursor'] as h}
      <label class="flex items-center gap-2">
        <input type="radio" name="harness" value={h} checked={harness === h} />
        <span>{h || 'any'}</span>
      </label>
    {/each}
  </fieldset>

  <fieldset>
    <legend class="mb-2 font-semibold">Platform</legend>
    {#each ['', 'darwin', 'linux', 'windows'] as p}
      <label class="flex items-center gap-2">
        <input type="radio" name="platform" value={p} checked={platform === p} />
        <span>{p || 'any'}</span>
      </label>
    {/each}
  </fieldset>

  <input type="hidden" name="category" value={category} />

  <button
    type="submit"
    class="rounded-md bg-neutral-900 px-3 py-1.5 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900"
  >
    Apply filters
  </button>
</form>
```

- [ ] **Step 3: Search page**

Create `web/src/routes/search/+page.svelte`:
```svelte
<script lang="ts">
  import type { PageData } from './$types';
  import SearchBar from '$lib/components/SearchBar.svelte';
  import ToolCard from '$lib/components/ToolCard.svelte';
  import FacetSidebar from '$lib/components/FacetSidebar.svelte';
  let { data }: { data: PageData } = $props();
</script>

<div class="mx-auto max-w-6xl px-6 py-8">
  <SearchBar value={data.q} />
</div>

<div class="mx-auto grid max-w-6xl grid-cols-1 gap-8 px-6 pb-12 md:grid-cols-[220px_1fr]">
  <aside>
    <FacetSidebar {...data} />
  </aside>
  <main>
    {#if data.error}
      <p class="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
        Search unavailable: {data.error}
      </p>
    {:else if !data.q}
      <p class="text-sm text-neutral-600 dark:text-neutral-400">Type a query to search.</p>
    {:else if data.results.length === 0}
      <p class="text-sm text-neutral-600 dark:text-neutral-400">No results for “{data.q}”.</p>
    {:else}
      <p class="mb-4 text-sm text-neutral-500">{data.results.length} results for “{data.q}”</p>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        {#each data.results as h}
          <ToolCard tool={h.tool} />
        {/each}
      </div>
    {/if}
  </main>
</div>
```

- [ ] **Step 4: Commit**

```bash
git add web/src/routes/search web/src/lib/components/FacetSidebar.svelte
git commit -m "feat(web): search page with facet sidebar"
```

---

## Task 7: Category page + docs shell

**Files:**
- Create: `web/src/routes/categories/[cat]/+page.server.ts`
- Create: `web/src/routes/categories/[cat]/+page.svelte`
- Create: `web/src/routes/docs/+page.svelte`

- [ ] **Step 1: Category load**

Create `web/src/routes/categories/[cat]/+page.server.ts`:
```ts
import type { PageServerLoad } from './$types';
import { getBrowse } from '$lib/api';

export const load: PageServerLoad = async ({ fetch, params }) => {
  const tools = await getBrowse(fetch, { category: params.cat });
  return { category: params.cat, tools };
};
```

- [ ] **Step 2: Category view**

Create `web/src/routes/categories/[cat]/+page.svelte`:
```svelte
<script lang="ts">
  import type { PageData } from './$types';
  import ToolCard from '$lib/components/ToolCard.svelte';
  let { data }: { data: PageData } = $props();
</script>

<section class="mx-auto max-w-6xl px-6 py-10">
  <h1 class="text-2xl font-semibold capitalize">{data.category}</h1>
  <p class="mt-1 text-sm text-neutral-500">{data.tools.length} tools</p>
  <div class="mt-6 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
    {#each data.tools as t}
      <ToolCard tool={t} />
    {/each}
  </div>
</section>
```

- [ ] **Step 3: Docs shell**

Create `web/src/routes/docs/+page.svelte`:
```svelte
<article class="prose mx-auto max-w-3xl px-6 py-10 dark:prose-invert">
  <h1>Agentpop docs</h1>
  <h2>Publishing a tool</h2>
  <p>
    Add an <code>agentpop.yaml</code> manifest to your tool's repository, then open a PR to the
    registry listing your repo URL.
  </p>
  <h2>CLI</h2>
  <pre><code>agentpop install &lt;slug&gt;
agentpop search &lt;query&gt;
agentpop doctor
</code></pre>
  <h2>Writing a manifest</h2>
  <p>See the project spec for the full schema.</p>
</article>
```

- [ ] **Step 4: Commit**

```bash
git add web/src/routes/categories web/src/routes/docs
git commit -m "feat(web): category page and docs shell"
```

---

## Task 8: Playwright smoke tests

**Files:**
- Create: `web/playwright.config.ts`
- Create: `web/tests/smoke.spec.ts`
- Modify: `web/package.json` (add test scripts)

- [ ] **Step 1: Install Playwright**

```bash
cd web
npm install -D @playwright/test
npx playwright install chromium
```

- [ ] **Step 2: Add a `test:e2e` script**

Edit `web/package.json` and add inside `"scripts"`:
```json
"test:e2e": "playwright test"
```

- [ ] **Step 3: Playwright config**

Create `web/playwright.config.ts`:
```ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'tests',
  timeout: 30_000,
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:5173'
  },
  webServer: {
    command: 'npm run dev -- --port 5173',
    port: 5173,
    reuseExistingServer: !process.env.CI,
    env: { AGENTPOP_API_URL: process.env.AGENTPOP_API_URL ?? 'http://localhost:8091' }
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }]
});
```

- [ ] **Step 4: The smoke test**

Create `web/tests/smoke.spec.ts`:
```ts
import { test, expect } from '@playwright/test';

// These tests assume bin/apid is running on :8091 against internal/api/testdata/corpus.
// Start it before `npm run test:e2e`.

test('home renders fixture tools', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toContainText('Find agentic tools');
  await expect(page.getByText('Tool A')).toBeVisible();
});

test('tool detail shows install tabs with both adapters', async ({ page }) => {
  await page.goto('/t/tool-a');
  await expect(page.locator('h1')).toContainText('Tool A');
  await expect(page.getByRole('button', { name: 'CLI' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Claude Code' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cursor' })).toBeVisible();
});

test('category page lists scoped tools', async ({ page }) => {
  await page.goto('/categories/search');
  await expect(page.locator('h1')).toContainText('search');
  await expect(page.getByText('Tool A')).toBeVisible();
});
```

- [ ] **Step 5: Run the smoke suite**

From the repo root:
```bash
bin/apid -addr :8091 -corpus internal/api/testdata/corpus -marrow http://127.0.0.1:1 &
APID=$!
cd web && AGENTPOP_API_URL=http://localhost:8091 npm run test:e2e
cd ..
kill $APID || true
```

Expect: all three tests pass.

- [ ] **Step 6: Commit**

```bash
git add web/playwright.config.ts web/tests web/package.json web/package-lock.json
git commit -m "test(web): Playwright smoke suite"
```

---

## Task 9: Build verification

- [ ] **Step 1: Production build**

```bash
cd web
npm run build
```

Expect success. SvelteKit writes to `.svelte-kit/output/` plus `build/` depending on adapter; the default `@sveltejs/adapter-auto` produces a Node server build.

- [ ] **Step 2: Type-check passes**

```bash
npm run check
```

Expect no errors.

- [ ] **Step 3: Commit any lockfile updates from prior steps**

From repo root:
```bash
git add -A
git diff --cached --quiet || git commit -m "chore(web): lockfile updates"
```

---

## Out of scope (deferred)

- Proper markdown rendering for READMEs (v1 renders as `<pre>`). Fast-follow: bring in `marked` or `markdown-it` and render to HTML inside `<article class="prose">`.
- Client-side dark-mode toggle button (we honour OS preference in v1).
- Pagination on search and category pages.
- Syntax highlighting for snippet code blocks.
- Analytics / view tracking.
