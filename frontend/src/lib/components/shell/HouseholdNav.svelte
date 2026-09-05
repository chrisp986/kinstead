<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';

	let { householdId }: { householdId: string } = $props();
	const items = [
		{ label: 'Report', icon: '⌂', path: '/households/[householdId]', key: 'report' },
		{ label: 'Calendar', icon: '◷', path: '/households/[householdId]/calendar', key: 'calendar' },
		{ label: 'Farm', icon: '⌁', path: '/households/[householdId]/farm', key: 'farm' },
		{ label: 'Work', icon: '⚒', path: '/households/[householdId]/work', key: 'work' },
		{ label: 'Trade', icon: '⇄', path: '/households/[householdId]/trade', key: 'trade' },
		{ label: 'Chronicle', icon: '≡', path: '/households/[householdId]/chronicle', key: 'chronicle' }
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
		<a
			href={resolve(item.path.replace('[householdId]', householdId) as `/households/${string}`)}
			aria-label={item.label}
			aria-current={active(item.path) ? 'page' : undefined}
			data-icon={item.icon}
			data-label={item.label}
		></a>
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
		visibility: visible;
		opacity: 1;
	}
	.household-nav a::before,
	.household-nav a::after {
		display: block;
		visibility: visible;
		opacity: 1;
	}
	.household-nav a::before {
		content: attr(data-icon);
		font-family: var(--font-display);
		font-size: 1.05rem;
		line-height: 1;
	}
	.household-nav a::after {
		content: attr(data-label);
		font: inherit;
		line-height: 1;
	}
	.household-nav a:hover,
	.household-nav a[aria-current='page'] {
		background: var(--surface-muted);
		color: var(--green);
	}
	@media (max-width: 767px) {
		.household-nav {
			position: fixed;
			z-index: 1000;
			right: 0;
			bottom: 0;
			left: 0;
			align-items: stretch;
			justify-content: space-around;
			gap: 0;
			min-height: var(--mobile-nav-height);
			padding: 0.25rem 0 max(0.25rem, env(safe-area-inset-bottom));
			border-top: 1px solid var(--line);
			border-bottom: 0;
			background: #f8f4e9;
			box-shadow: 0 -3px 14px rgba(48, 43, 31, 0.14);
			transform: translateZ(0);
			-webkit-transform: translateZ(0);
		}
		.household-nav a {
			flex: 1 1 0;
			min-width: 0;
			min-height: var(--tap-min);
			padding: 0.35rem 0.1rem;
			flex-direction: column;
			gap: 0.22rem;
			color: #45473f;
			font-size: 0.64rem;
			letter-spacing: 0;
			text-transform: none;
			-webkit-text-fill-color: currentColor;
		}
		.household-nav a[aria-current='page'] {
			color: var(--green);
		}
		.household-nav a::before {
			font-size: 1.25rem;
		}
	}
	@media (max-width: 390px) {
		.household-nav a {
			font-size: 0.58rem;
		}
		.household-nav a::before {
			font-size: 1.15rem;
		}
	}
</style>
