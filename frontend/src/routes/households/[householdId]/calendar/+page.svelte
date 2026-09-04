<script lang="ts">
	import PageHeader from '$lib/components/shell/PageHeader.svelte';
	import {
		calendarForGameDay,
		formatCalendarPosition,
		formatPhaseName,
		formatRelativeGameDay
	} from '$lib/domain/time';

	let { data } = $props();
	let filter = $state('all');
	const filters = [
		['all', 'All'],
		['trade', 'My obligations'],
		['farm', 'Farm'],
		['festivals', 'Festivals & assemblies'],
		['politics', 'Politics']
	] as const;
	let filteredEvents = $derived(
		filter === 'all'
			? data.calendar.events
			: data.calendar.events.filter((event) => event.category === filter)
	);
	function eventPosition(gameDay: number): string {
		const date = calendarForGameDay(gameDay);
		return formatCalendarPosition(date.phase ?? date.seasonal_phase, date.week_of_half);
	}
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
			<h2>
				{formatCalendarPosition(data.calendar.calendar.phase, data.calendar.calendar.week_of_half)}
			</h2>
			<p>
				{formatPhaseName(data.calendar.calendar.production_season)} · {data.calendar.calendar
					.half_year} half
			</p>
		</div>
		<div class="cycle">
			<span>Year cycle</span>
			<strong>{formatPhaseName(data.calendar.calendar.production_season)}</strong>
			<small
				>{data.calendar.next_half_year.type} half begins {formatRelativeGameDay(
					data.calendar.current_game_day,
					data.calendar.next_half_year.game_day
				)}</small
			>
		</div>
	</section>
	<section aria-labelledby="upcoming-heading">
		<div class="section-heading">
			<h2 id="upcoming-heading">Upcoming</h2>
			<span>{filteredEvents.length} events</span>
		</div>
		<nav class="filters" aria-label="Calendar filters">
			{#each filters as [value, label] (value)}<button
					class:active={filter === value}
					type="button"
					onclick={() => (filter = value)}>{label}</button
				>{/each}
		</nav>
		{#if filteredEvents.length === 0}
			<p class="empty">No events match this view.</p>
		{:else}
			<ul>
				{#each filteredEvents as event (event.id)}
					<li class:action={event.action_required}>
						<div>
							<strong
								>{formatRelativeGameDay(data.calendar.current_game_day, event.game_day)}</strong
							><small>{eventPosition(event.game_day)}</small>
						</div>
						<span>{event.title}</span>
					</li>
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
	.section-heading {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
	}
	.section-heading > span {
		color: var(--ink-soft);
		font-size: 0.78rem;
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
	.filters {
		display: flex;
		gap: 0.4rem;
		margin: 0.8rem 0;
		overflow-x: auto;
	}
	.filters button {
		border: 1px solid var(--line-light);
		padding: 0.55rem 0.7rem;
		background: var(--surface);
		color: var(--ink-soft);
		font: inherit;
		font-size: 0.76rem;
		white-space: nowrap;
		cursor: pointer;
	}
	.filters button.active,
	.filters button:hover,
	.filters button:focus-visible {
		border-color: var(--green);
		background: var(--green);
		color: white;
	}
	li {
		display: flex;
		gap: 1rem;
		padding: 0.8rem;
		border-bottom: 1px solid var(--line-light);
	}
	li.action {
		border-left: 3px solid var(--critical);
	}
	li strong {
		display: block;
		min-width: 7rem;
		color: var(--ink-soft);
	}
	li small {
		display: block;
		margin-top: 0.2rem;
		color: var(--ink-soft);
		font-size: 0.7rem;
	}
	.empty {
		padding: 1rem;
		background: var(--surface-muted);
	}
</style>
