<script lang="ts">
	import type { ChronicleEntry } from '$lib/api/generated';
	import { describeChronicleEntry } from '$lib/domain/chronicle';
	import { calendarForGameDay, formatCalendarPosition } from '$lib/domain/time';

	let { entries, currentGameDay }: { entries: ChronicleEntry[]; currentGameDay: number } = $props();
	let grouped = $derived.by(() => {
		const groups: { gameDay: number; entries: ChronicleEntry[] }[] = [];
		for (const entry of entries) {
			const current = groups.at(-1);
			if (current?.gameDay === entry.occurred_game_day) current.entries.push(entry);
			else groups.push({ gameDay: entry.occurred_game_day, entries: [entry] });
		}
		return groups;
	});
	function delta(entry: ChronicleEntry): number {
		const value = entry.data.trust_delta ?? entry.data.standing_delta;
		return typeof value === 'number' ? value : 0;
	}
</script>

<section class="panel chronicle-panel" aria-labelledby="chronicle-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Household memory</p>
			<h2 id="chronicle-heading">What happened</h2>
		</div>
		<span class="count">{entries.length}</span>
	</div>

	{#if entries.length === 0}
		<p class="empty">No household events have been recorded yet.</p>
	{:else}
		<ol class="timeline">
			{#each grouped as group (group.gameDay)}
				{@const date = calendarForGameDay(group.gameDay)}
				<li class="day-group">
					<div class="day-marker">
						<span>{group.gameDay === currentGameDay ? 'Today' : 'Recorded'}</span><strong
							>{formatCalendarPosition(
								date.phase || date.seasonal_phase || date.production_season,
								date.week_of_half
							)}</strong
						>
					</div>
					<ol class="day-events">
						{#each group.entries as entry (entry.id)}
							{@const description = describeChronicleEntry(entry)}
							<li class:positive={delta(entry) > 0} class:negative={delta(entry) < 0}>
								<div class="event-copy">
									<h3>{description.title}</h3>
									<p>{description.detail}</p>
								</div>
							</li>
						{/each}
					</ol>
				</li>
			{/each}
		</ol>
	{/if}
</section>

<style>
	.chronicle-panel {
		grid-column: 1 / -1;
	}
	.count {
		display: grid;
		place-items: center;
		min-width: 2rem;
		height: 2rem;
		padding: 0 0.45rem;
		border-radius: 1rem;
		background: var(--ink);
		color: var(--paper);
		font-weight: 700;
	}
	.timeline {
		display: grid;
		margin: 1.25rem 0 0;
		padding: 0;
		list-style: none;
	}
	.day-group {
		display: grid;
		grid-template-columns: 3.5rem 1fr;
		gap: 1rem;
		padding: 0.9rem 0;
		border-top: 1px solid var(--line-light);
	}
	.day-group:first-child {
		border-top: 0;
	}
	.day-events {
		display: grid;
		gap: 0.7rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.day-events li {
		min-width: 0;
	}
	.day-marker {
		display: grid;
		align-content: center;
		justify-items: center;
		min-height: 3.25rem;
		background: var(--surface-muted);
		color: var(--ink-soft);
	}
	.day-marker span {
		font-size: 0.62rem;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.day-marker strong {
		color: var(--ink);
		font-family: var(--font-display);
		font-size: 0.9rem;
		text-align: center;
		line-height: 1;
	}
	.event-copy {
		display: grid;
		align-content: center;
	}
	.event-copy h3 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1.08rem;
	}
	.event-copy p {
		margin: 0.2rem 0 0;
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.timeline li.positive h3 {
		color: var(--positive);
	}
	.timeline li.negative h3 {
		color: var(--critical);
	}
</style>
