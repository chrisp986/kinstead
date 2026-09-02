<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import { resolve } from '$app/paths';
	import { env } from '$env/dynamic/public';
	import './global.css';

	let { children } = $props();
	let displayName = $derived(env.PUBLIC_GAME_NAME ?? 'Household Chronicle');
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="shell">
	<nav aria-label="Primary navigation">
		<a class="brand" href={resolve('/')}>{displayName}</a>
		<span>Household simulation</span>
	</nav>
	{@render children()}
</div>
