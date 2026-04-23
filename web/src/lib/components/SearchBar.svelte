<script lang="ts">
	interface Props {
		value?: string;
		autofocus?: boolean;
	}
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
