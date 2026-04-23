<script lang="ts">
	import type { PageData } from './$types';
	import ToolCard from '$lib/components/ToolCard.svelte';
	let { data }: { data: PageData } = $props();
</script>

<section class="mx-auto max-w-6xl px-6 py-10">
	<h1 class="text-2xl font-semibold capitalize">{data.category}</h1>
	<p class="mt-1 text-sm text-neutral-500">{data.tools.length} tools</p>
	<div class="mt-6 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
		{#each data.tools as t (t.slug)}
			<ToolCard tool={t} />
		{/each}
	</div>
</section>
