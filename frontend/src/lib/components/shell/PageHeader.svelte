<script lang="ts">
	let {
		eyebrow,
		title,
		description = ''
	}: { eyebrow?: string; title: string; description?: string } = $props();
</script>

<header class="page-header">
	{#if eyebrow}<p class="eyebrow">{eyebrow}</p>{/if}
	<h2>{title}</h2>
	{#if description}<p>{description}</p>{/if}
</header>

<style>
	.page-header {
		padding: 1.25rem 0 0.5rem;
	}
	h2 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: clamp(1.7rem, 5vw, 2.2rem);
		font-weight: 500;
	}
	.page-header > p:last-child {
		max-width: 44rem;
		margin: 0.4rem 0 0;
		color: var(--ink-soft);
		font-size: 0.88rem;
	}
</style>
