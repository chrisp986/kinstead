<script lang="ts">
	import { SvelteMap } from 'svelte/reactivity';
	import type { CalendarEvent } from '$lib/api/generated';
	import PageHeader from '$lib/components/shell/PageHeader.svelte';
	import { formatMilli, labelResource } from '$lib/domain/format';
	import { calendarEventLabel } from '$lib/domain/calendar';
	import {
		calendarForGameDay,
		calendarGroupForEvent,
		type CalendarGroup,
		formatCalendarPosition,
		formatPhaseName,
		formatRelativeGameDay,
		settingYear
	} from '$lib/domain/time';

	let { data } = $props();
	let filter = $state('all');
	let view = $state<'upcoming' | 'cycle'>('upcoming');
	const filters = [
		['all', 'All'],
		['season', 'Seasons'],
		['contract', 'Contracts'],
		['shipment', 'Shipments'],
		['farm', 'Farm'],
		['world', 'World'],
		['politics', 'Politics']
	] as const;
	const groupOrder: CalendarGroup[] = [
		'urgent',
		'today',
		'this_week',
		'next_week',
		'later_current_half',
		'next_half',
		'later'
	];
	const groupLabels: Record<CalendarGroup, string> = {
		urgent: 'Urgent',
		today: 'Today',
		this_week: 'This week',
		next_week: 'Next week',
		later_current_half: 'Later this half-year',
		next_half: 'Next half-year',
		later: 'Later'
	};

	function matchesFilter(event: CalendarEvent): boolean {
		if (filter === 'all') return true;
		return event.category === filter;
	}

	let filteredEvents = $derived(
		data.calendar.events.filter(matchesFilter).toSorted((a, b) => {
			const aAction = a.action_required ? 0 : 1;
			const bAction = b.action_required ? 0 : 1;
			if (aAction !== bAction) return aAction - bAction;
			if (a.game_day !== b.game_day) return a.game_day - b.game_day;
			const importance = { critical: 0, important: 1, context: 2 };
			if (importance[a.importance] !== importance[b.importance]) {
				return importance[a.importance] - importance[b.importance];
			}
			return a.id.localeCompare(b.id);
		})
	);

	let groupedEvents = $derived.by(() => {
		const groups = new SvelteMap<CalendarGroup, CalendarEvent[]>();
		for (const event of filteredEvents) {
			const group = calendarGroupForEvent(
				data.calendar.current_game_day,
				event.game_day,
				event.action_required,
				event.importance
			);
			const events = groups.get(group) ?? [];
			events.push(event);
			groups.set(group, events);
		}
		return groupOrder
			.filter((key) => groups.has(key))
			.map((key) => ({ key, label: groupLabels[key], events: groups.get(key) ?? [] }));
	});

	type CycleRow = {
		offset: number;
		label: string;
		half: 'summer' | 'winter';
		kind: 'phase' | 'season' | 'festival' | 'harvest' | 'assembly';
		phase?: string;
	};
	const cycleRows: CycleRow[] = [
		{ offset: 0, label: 'Spring', half: 'summer', kind: 'phase', phase: 'spring' },
		{ offset: 91, label: 'Summer begins', half: 'summer', kind: 'season' },
		{ offset: 91, label: 'Early summer', half: 'summer', kind: 'phase', phase: 'early_summer' },
		{ offset: 121, label: 'Midsummer', half: 'summer', kind: 'festival' },
		{ offset: 121, label: 'High summer', half: 'summer', kind: 'phase', phase: 'high_summer' },
		{ offset: 152, label: 'Harvest begins', half: 'summer', kind: 'harvest' },
		{ offset: 152, label: 'Late summer', half: 'summer', kind: 'phase', phase: 'late_summer' },
		{ offset: 182, label: 'Autumn', half: 'winter', kind: 'phase', phase: 'autumn' },
		{ offset: 273, label: 'Winter begins', half: 'winter', kind: 'season' },
		{ offset: 273, label: 'Early winter', half: 'winter', kind: 'phase', phase: 'early_winter' },
		{ offset: 287, label: 'Þing', half: 'winter', kind: 'assembly' },
		{ offset: 304, label: 'Midwinter', half: 'winter', kind: 'festival', phase: 'midwinter' },
		{ offset: 320, label: 'Jól', half: 'winter', kind: 'festival' },
		{ offset: 334, label: 'Late winter', half: 'winter', kind: 'phase', phase: 'late_winter' }
	];

	function eventPosition(gameDay: number): string {
		const date = calendarForGameDay(gameDay);
		return formatCalendarPosition(
			date.phase || date.seasonal_phase || date.production_season,
			date.week_of_half
		);
	}

	function eventDetail(event: CalendarEvent): string {
		const parts: string[] = [];
		if (event.quantity_milli !== undefined && event.resource_type) {
			parts.push(`${formatMilli(event.quantity_milli)} ${labelResource(event.resource_type)}`);
		}
		if (event.counterparty_household_name) parts.push(`with ${event.counterparty_household_name}`);
		return parts.join(' · ');
	}

	function cycleDay(offset: number): number {
		return Math.floor(data.calendar.current_game_day / 364) * 364 + offset;
	}

	function cycleRowIsNow(row: CycleRow): boolean {
		const current = calendarForGameDay(data.calendar.current_game_day);
		const day = ((data.calendar.current_game_day % 364) + 364) % 364;
		if (row.phase) return (current.phase || current.production_season) === row.phase;
		return day === row.offset;
	}

	function cycleEvents(offset: number): CalendarEvent[] {
		const day = cycleDay(offset);
		return filteredEvents.filter((event) => event.game_day === day);
	}

	function rowsForHalf(half: 'summer' | 'winter'): CycleRow[] {
		return cycleRows.filter((row) => row.half === half);
	}
</script>

<svelte:head><title>{data.report.household_name} · Calendar</title></svelte:head>

<main class="calendar-page">
	<PageHeader
		eyebrow="World rhythm"
		title="Calendar"
		description="A planning view for seasons, obligations, and household events."
	/>
	<section class="current" aria-labelledby="current-heading">
		<div>
			<p class="eyebrow" id="current-heading">Today</p>
			<h2>
				{settingYear(data.calendar.setting_start_year, data.calendar.calendar)} CE ·
				{formatCalendarPosition(
					data.calendar.calendar.phase ||
						data.calendar.calendar.seasonal_phase ||
						data.calendar.calendar.production_season,
					data.calendar.calendar.week_of_half
				)}
			</h2>
			<p>
				{formatPhaseName(data.calendar.calendar.production_season)} · {formatPhaseName(
					data.calendar.calendar.half_year
				)} half
			</p>
		</div>
		<div class="cycle-summary">
			<span>Next boundary</span>
			<strong>{formatPhaseName(data.calendar.next_half_year.type)} half</strong>
			<small
				>{formatRelativeGameDay(
					data.calendar.current_game_day,
					data.calendar.next_half_year.game_day
				)}
				until it begins</small
			>
		</div>
	</section>

	<div class="view-tabs" role="tablist" aria-label="Calendar views">
		<button
			class:active={view === 'upcoming'}
			role="tab"
			type="button"
			aria-selected={view === 'upcoming'}
			onclick={() => (view = 'upcoming')}>Upcoming</button
		>
		<button
			class:active={view === 'cycle'}
			role="tab"
			type="button"
			aria-selected={view === 'cycle'}
			onclick={() => (view = 'cycle')}>Year cycle</button
		>
	</div>

	{#if view === 'upcoming'}
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
				<div class="event-groups">
					{#each groupedEvents as group (group.key)}
						<section class="event-group" aria-labelledby={`group-${group.key}`}>
							<h3 id={`group-${group.key}`}>{group.label}</h3>
							<ul class="event-list">
								{#each group.events as event (event.id)}
									<li id={`event-${event.id}`} class:action={event.action_required}>
										<div class="event-time">
											<strong
												>{formatRelativeGameDay(
													data.calendar.current_game_day,
													event.game_day
												)}</strong
											>
											<small>{eventPosition(event.game_day)}</small>
										</div>
										<div class="event-copy">
											<strong>{calendarEventLabel(event)}</strong>
											{#if eventDetail(event)}<small>{eventDetail(event)}</small>{/if}
										</div>
									</li>
								{/each}
							</ul>
						</section>
					{/each}
				</div>
			{/if}
		</section>
	{:else}
		<section class="year-cycle" aria-labelledby="year-cycle-heading">
			<div class="section-heading">
				<div>
					<h2 id="year-cycle-heading">Year cycle</h2>
					<p>Seasonal phases, gatherings, and work markers.</p>
				</div>
			</div>
			<div class="cycle-halves">
				{#each ['summer', 'winter'] as half (half)}
					<section class="cycle-half" aria-labelledby={`half-${half}`}>
						<h3 id={`half-${half}`}>{formatPhaseName(half)} half</h3>
						<ol class="cycle-timeline">
							{#each rowsForHalf(half as 'summer' | 'winter') as row (`${half}-${row.offset}-${row.label}`)}
								{@const events = cycleEvents(row.offset)}
								<li class:now={cycleRowIsNow(row)} class={`kind-${row.kind}`}>
									<div class="cycle-line" aria-hidden="true"></div>
									<div class="cycle-copy">
										<strong>{row.label}</strong>
										{#if cycleRowIsNow(row)}<span class="now-label">Now</span>{/if}
										{#each events as event (event.id)}
											<small class:action-text={event.action_required}
												>{calendarEventLabel(event)}</small
											>
										{/each}
									</div>
								</li>
							{/each}
						</ol>
					</section>
				{/each}
			</div>
		</section>
	{/if}
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
	.cycle-summary {
		display: grid;
		align-content: center;
		justify-items: end;
	}
	.cycle-summary span,
	.cycle-summary small {
		color: var(--ink-soft);
		font-size: 0.75rem;
	}
	.cycle-summary strong {
		color: var(--green);
		font-family: var(--font-display);
		font-size: 1.35rem;
	}
	.view-tabs {
		display: flex;
		gap: 0.35rem;
		border-bottom: 1px solid var(--line);
	}
	.view-tabs button {
		border: 0;
		border-bottom: 3px solid transparent;
		padding: 0.7rem 0.9rem;
		background: transparent;
		color: var(--ink-soft);
		font: inherit;
		font-weight: 700;
		cursor: pointer;
	}
	.view-tabs button.active,
	.view-tabs button:hover,
	.view-tabs button:focus-visible {
		border-bottom-color: var(--green);
		color: var(--green);
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
	.event-groups {
		display: grid;
		gap: 1.1rem;
	}
	.event-group h3 {
		margin: 0;
		font-size: 0.75rem;
		letter-spacing: 0.09em;
		text-transform: uppercase;
	}
	.event-list {
		display: grid;
		gap: 0.5rem;
		margin: 0.45rem 0 0;
		padding: 0;
		list-style: none;
	}
	.event-list li {
		display: grid;
		grid-template-columns: minmax(7rem, 0.4fr) minmax(0, 1fr);
		gap: 1rem;
		padding: 0.8rem;
		border-bottom: 1px solid var(--line-light);
	}
	.event-list li.action {
		border-left: 3px solid var(--critical);
	}
	.event-time strong,
	.event-time small,
	.event-copy strong,
	.event-copy small {
		display: block;
	}
	.event-time strong {
		color: var(--ink-soft);
	}
	.event-time small,
	.event-copy small {
		margin-top: 0.2rem;
		color: var(--ink-soft);
		font-size: 0.78rem;
	}
	.event-copy strong {
		font-family: var(--font-display);
		font-size: 1.05rem;
	}
	.empty {
		padding: 1rem;
		background: var(--surface-muted);
	}
	.year-cycle > .section-heading p {
		margin: 0.3rem 0 0;
	}
	.cycle-halves {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
	}
	.cycle-half {
		padding: 1rem;
		background: var(--surface-muted);
		border: 1px solid var(--line-light);
	}
	.cycle-half h3 {
		margin: 0 0 0.7rem;
		color: var(--green);
		font-family: var(--font-display);
		font-size: 1.2rem;
		font-weight: 500;
		text-transform: capitalize;
	}
	.cycle-timeline {
		display: grid;
		gap: 0.15rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.cycle-timeline li {
		position: relative;
		display: grid;
		grid-template-columns: 0.75rem 1fr;
		gap: 0.7rem;
		min-height: 2.25rem;
	}
	.cycle-line {
		position: relative;
		width: 0.55rem;
		margin-top: 0.25rem;
		border-radius: 50%;
		background: var(--line);
	}
	.cycle-timeline li:not(:last-child) .cycle-line::after {
		position: absolute;
		top: 0.5rem;
		left: 0.23rem;
		width: 1px;
		height: 2.25rem;
		background: var(--line);
		content: '';
	}
	.cycle-copy {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.45rem;
		padding-bottom: 0.35rem;
	}
	.cycle-copy strong {
		font-family: var(--font-display);
		font-size: 0.98rem;
		font-weight: 500;
	}
	.cycle-copy small {
		flex-basis: 100%;
		color: var(--ink-soft);
		font-size: 0.76rem;
	}
	.cycle-timeline li.kind-festival .cycle-copy strong,
	.cycle-timeline li.kind-assembly .cycle-copy strong {
		color: var(--ochre);
	}
	.cycle-timeline li.kind-harvest .cycle-copy strong {
		color: var(--green);
	}
	.cycle-timeline li.now .cycle-line {
		background: var(--green);
		box-shadow: 0 0 0 4px rgba(67, 98, 67, 0.15);
	}
	.now-label {
		padding: 0.15rem 0.35rem;
		background: var(--green);
		color: white;
		font-size: 0.63rem;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.action-text {
		color: var(--critical) !important;
		font-weight: 700;
	}
	@media (max-width: 700px) {
		.current,
		.cycle-halves {
			grid-template-columns: 1fr;
		}
		.current {
			display: grid;
		}
		.cycle-summary {
			justify-items: start;
		}
	}
	@media (max-width: 420px) {
		.event-list li {
			grid-template-columns: 1fr;
			gap: 0.3rem;
		}
	}
</style>
