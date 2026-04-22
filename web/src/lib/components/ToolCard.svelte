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
