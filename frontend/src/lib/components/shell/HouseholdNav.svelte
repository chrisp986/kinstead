<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';

	let { householdId }: { householdId: string } = $props();
	const items = [
		{ label: 'Report', path: '/households/[householdId]', key: 'report' },
		{ label: 'Farm', path: '/households/[householdId]/farm', key: 'farm' },
		{ label: 'Work', path: '/households/[householdId]/work', key: 'work' },
		{ label: 'Trade', path: '/households/[householdId]/trade', key: 'trade' },
		{ label: 'Chronicle', path: '/households/[householdId]/chronicle', key: 'chronicle' }
	] as const;

	function href(path: (typeof items)[number]['path']): string {
		return resolve(path.replace('[householdId]', householdId) as `/households/${string}`);
	}

	function active(path: (typeof items)[number]['path']): boolean {
		const current = page.url.pathname;
		const root = href('/households/[householdId]');
		if (path === '/households/[householdId]') return current === root;
		return current.startsWith(href(path));
	}
</script>

<nav class="household-nav" aria-label="Household sections">
	{#each items as item (item.key)}
		<!-- href() applies SvelteKit's base path to a typed dynamic route. -->
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
		<a
			href={href(item.path)}
			aria-label={item.label}
			aria-current={active(item.path) ? 'page' : undefined}
		>
			<span class="icon" aria-hidden="true">
				{#if item.key === 'report'}⌂{:else if item.key === 'farm'}⌁{:else if item.key === 'work'}✦{:else if item.key === 'trade'}⇄{:else}≡{/if}
			</span>
			<span>{item.label}</span>
		</a>
	{/each}
</nav>

<style>
	.household-nav {
		display: flex;
		gap: 0.25rem;
		padding: 0.5rem 0;
		border-bottom: 1px solid var(--line-light);
	}
	.household-nav a {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.35rem;
		min-height: var(--tap-min);
		padding: 0.45rem 0.75rem;
		color: var(--ink-soft);
		font-size: 0.78rem;
		font-weight: 700;
		text-decoration: none;
	}
	.household-nav a:hover,
	.household-nav a[aria-current='page'] {
		background: var(--surface-muted);
		color: var(--green);
	}
	.icon {
		font-family: var(--font-display);
		font-size: 1.05rem;
		line-height: 1;
	}
	@media (max-width: 767px) {
		.household-nav {
			position: fixed;
			z-index: 20;
			right: 0;
			bottom: 0;
			left: 0;
			justify-content: space-around;
			gap: 0;
			padding: 0.25rem 0 max(0.25rem, env(safe-area-inset-bottom));
			border-top: 1px solid var(--line);
			border-bottom: 0;
			background: rgba(248, 244, 233, 0.97);
			box-shadow: 0 -3px 14px rgba(48, 43, 31, 0.1);
		}
		.household-nav a {
			flex: 1;
			min-width: 3.6rem;
			min-height: var(--tap-min);
			padding: 0.35rem 0.15rem;
			flex-direction: column;
			gap: 0.15rem;
			font-size: 0.66rem;
		}
		.icon {
			font-size: 1.1rem;
		}
	}
</style>
