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
	@media (max-width: 390px) {
		.page-header {
			padding: 0.8rem 0 0.35rem;
		}
		h2 {
			font-size: 1.5rem;
			line-height: 1.05;
		}
		.page-header > p:last-child {
			margin-top: 0.3rem;
			font-size: 0.78rem;
			line-height: 1.35;
		}
	}
</style>
