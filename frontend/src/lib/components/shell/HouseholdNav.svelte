<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';

	let { householdId }: { householdId: string } = $props();
	const items = [
		{ label: 'Report', path: '/households/[householdId]', key: 'report' },
		{ label: 'Calendar', path: '/households/[householdId]/calendar', key: 'calendar' },
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
		<a
			href={resolve(item.path.replace('[householdId]', householdId) as `/households/${string}`)}
			aria-label={item.label}
			aria-current={active(item.path) ? 'page' : undefined}
		>
			{#if item.key === 'report'}
				<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
					<path d="M3 11.5 12 4l9 7.5" />
					<path d="M5.5 10.5V20h13v-9.5" />
					<path d="M9.5 20v-6h5v6" />
				</svg>
			{:else if item.key === 'calendar'}
				<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
					<rect x="3.5" y="5.5" width="17" height="15" rx="1.5" />
					<path d="M8 3v5M16 3v5M3.5 10h17" />
					<path d="M8 14h2M14 14h2M8 17h2M14 17h2" />
				</svg>
			{:else if item.key === 'farm'}
				<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
					<path d="M12 21V10" />
					<path d="M12 13c-4.5 0-7-2.5-7-6 4.5 0 7 2.5 7 6Z" />
					<path d="M12 16c4.5 0 7-2.5 7-6-4.5 0-7 2.5-7 6Z" />
				</svg>
			{:else if item.key === 'work'}
				<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
					<path d="m14.5 5.5 4 4" />
					<path d="m6 18 9.5-9.5" />
					<path d="m4.5 16.5 3 3" />
					<path d="M13 4.5c2.2-1.6 4.7-1.4 6.5.4L16 8.4Z" />
				</svg>
			{:else if item.key === 'trade'}
				<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
					<path d="M4 8h14" />
					<path d="m15 5 3 3-3 3" />
					<path d="M20 16H6" />
					<path d="m9 13-3 3 3 3" />
				</svg>
			{:else}
				<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
					<path d="M5 4.5h10.5A3.5 3.5 0 0 1 19 8v11.5H8.5A3.5 3.5 0 0 1 5 16Z" />
					<path d="M5 16a3.5 3.5 0 0 1 3.5-3.5H19" />
					<path d="M9 8h6" />
				</svg>
			{/if}
			<small class="label">{item.label}</small>
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
		display: block;
		width: 1.1rem;
		height: 1.1rem;
		flex: 0 0 auto;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.8;
	}
	.label {
		display: block;
		color: currentColor;
		font: inherit;
		font-size: inherit;
		line-height: 1;
	}
	@media (max-width: 767px) {
		.household-nav {
			position: fixed;
			z-index: 100;
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
			background: var(--paper);
			box-shadow: 0 -3px 14px rgba(48, 43, 31, 0.1);
		}
		.household-nav a {
			flex: 1 1 0;
			min-width: 0;
			min-height: var(--tap-min);
			padding: 0.35rem 0.1rem;
			flex-direction: column;
			gap: 0.22rem;
			color: #57584f;
			font-size: 0.64rem;
			letter-spacing: 0;
			text-transform: none;
		}
		.household-nav a[aria-current='page'] {
			color: var(--green);
		}
		.icon {
			width: 1.25rem;
			height: 1.25rem;
			stroke-width: 2;
		}
	}
	@media (max-width: 390px) {
		.household-nav a {
			font-size: 0.58rem;
		}
		.icon {
			width: 1.15rem;
			height: 1.15rem;
		}
	}
</style>
