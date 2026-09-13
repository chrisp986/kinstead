<script lang="ts">
	let { title, children }: { title: string; children: import('svelte').Snippet } = $props();
</script>

<section class="panel">
	<div class="section-heading"><h2>{title}</h2></div>
	{@render children()}
</section>
