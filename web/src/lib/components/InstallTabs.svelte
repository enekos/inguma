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
