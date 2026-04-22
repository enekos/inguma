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
