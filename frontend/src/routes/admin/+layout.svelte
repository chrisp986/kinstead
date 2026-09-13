<script lang="ts">
	import { resolve } from '$app/paths';
	let { children } = $props();
</script>

<svelte:head><title>Developer console</title></svelte:head>

<main class="admin-shell">
	<nav class="admin-nav" aria-label="Developer console">
		<a href={resolve('/admin')}>Console overview</a>
		<a href={resolve('/admin/accounts')}>Accounts</a>
		<a href={resolve('/admin/errors')}>Errors</a>
	</nav>
	{@render children()}
</main>

<style>
	.admin-shell {
		display: grid;
		gap: 1rem;
		padding: 1rem 0 2rem;
	}
	.admin-nav {
		justify-content: flex-start;
		gap: 1rem;
		flex-wrap: wrap;
	}
	.admin-nav a {
		color: var(--green);
		font-weight: 800;
		text-decoration: none;
	}
</style>
