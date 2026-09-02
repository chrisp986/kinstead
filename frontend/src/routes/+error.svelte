<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
</script>

<svelte:head><title>Unable to load household</title></svelte:head>
<main class="error-page">
	<p class="eyebrow">Something went wrong</p>
	<h1>{page.status}</h1>
	<p>{page.error?.message ?? 'The household could not be loaded.'}</p>
	<a href={resolve('/')}>Try again</a>
</main>

<style>
	.error-page {
		max-width: 35rem;
		padding: 6rem 0;
	}
	h1 {
		margin: 0.5rem 0;
		font-family: var(--font-display);
		font-size: 5rem;
	}
	p {
		color: var(--ink-soft);
	}
	a {
		display: inline-block;
		margin-top: 1rem;
		color: var(--green);
		font-weight: 700;
	}
</style>
