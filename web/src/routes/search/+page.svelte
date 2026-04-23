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
			<p
				class="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300"
			>
				Search unavailable: {data.error}
			</p>
		{:else if !data.q}
			<p class="text-sm text-neutral-600 dark:text-neutral-400">Type a query to search.</p>
		{:else if data.results.length === 0}
			<p class="text-sm text-neutral-600 dark:text-neutral-400">No results for “{data.q}”.</p>
		{:else}
			<p class="mb-4 text-sm text-neutral-500">{data.results.length} results for “{data.q}”</p>
			<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
				{#each data.results as h (h.tool.slug)}
					<ToolCard tool={h.tool} />
				{/each}
			</div>
		{/if}
	</main>
</div>
