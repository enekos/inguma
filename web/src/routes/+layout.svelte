<script lang="ts">
  import '../app.css';
  import Nav from '$lib/components/Nav.svelte';
  let { children } = $props();
</script>

<div class="min-h-screen bg-white text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
  <Nav />
  {@render children()}
</div>
