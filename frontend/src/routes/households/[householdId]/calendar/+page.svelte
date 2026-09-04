<script lang="ts">
	import PageHeader from '$lib/components/shell/PageHeader.svelte';
	import { formatGameDay, formatPhase } from '$lib/domain/time';

	let { data } = $props();
</script>

<svelte:head><title>{data.report.household_name} · Calendar</title></svelte:head>

<main class="calendar-page">
	<PageHeader
		eyebrow="World rhythm"
		title="Calendar"
		description="A year of seasons, obligations, and household events measured in world days."
	/>
	<section class="current" aria-labelledby="current-heading">
		<div>
			<p class="eyebrow" id="current-heading">Today</p>
			<h2>{formatGameDay(data.calendar.current)}</h2>
			<p>{formatPhase(data.calendar.current)} · day {data.calendar.current.day_of_year + 1}</p>
		</div>
		<div class="cycle">
			<span>Year cycle</span>
			<strong>{data.calendar.current.production_season}</strong>
			<small>{data.calendar.current.half_year} half</small>
		</div>
	</section>
	<section aria-labelledby="upcoming-heading">
		<h2 id="upcoming-heading">Upcoming</h2>
		{#if data.calendar.events.length === 0}
			<p class="empty">No seasonal changes in this window.</p>
		{:else}
			<ul>
				{#each data.calendar.events as event (event.id)}
					<li><strong>Day {event.game_day}</strong><span>{event.title}</span></li>
				{/each}
			</ul>
		{/if}
	</section>
</main>

<style>
	.calendar-page {
		display: grid;
		gap: 1rem;
	}
	.current {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.25rem;
		background: var(--surface-muted);
		border: 1px solid var(--line-light);
	}
	h2 {
		margin: 0.15rem 0;
		font-family: var(--font-display);
		font-weight: 500;
	}
	p {
		color: var(--ink-soft);
	}
	.cycle {
		display: grid;
		align-content: center;
		justify-items: end;
		text-transform: capitalize;
	}
	.cycle span,
	.cycle small {
		color: var(--ink-soft);
		font-size: 0.75rem;
	}
	.cycle strong {
		color: var(--green);
		font-family: var(--font-display);
		font-size: 1.35rem;
	}
	ul {
		display: grid;
		gap: 0.5rem;
		padding: 0;
		list-style: none;
	}
	li {
		display: flex;
		gap: 1rem;
		padding: 0.8rem;
		border-bottom: 1px solid var(--line-light);
	}
	li strong {
		min-width: 4rem;
		color: var(--ink-soft);
	}
	.empty {
		padding: 1rem;
		background: var(--surface-muted);
	}
</style>
