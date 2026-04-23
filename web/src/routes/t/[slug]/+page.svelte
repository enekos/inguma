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
