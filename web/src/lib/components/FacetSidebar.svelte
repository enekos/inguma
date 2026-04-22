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
