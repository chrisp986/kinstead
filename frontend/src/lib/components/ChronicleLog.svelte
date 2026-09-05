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
		min-width: 0;
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
		min-width: 0;
		margin: 1.25rem 0 0;
		padding: 0;
		list-style: none;
	}
	.day-group {
		display: grid;
		grid-template-columns: 7rem minmax(0, 1fr);
		gap: 1rem;
		min-width: 0;
		padding: 0.9rem 0;
		border-top: 1px solid var(--line-light);
	}
	.day-group:first-child {
		border-top: 0;
	}
	.day-events {
		display: grid;
		gap: 0.7rem;
		min-width: 0;
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
		min-width: 0;
		min-height: 3.25rem;
		padding: 0.55rem 0.45rem;
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
		max-width: 100%;
		color: var(--ink);
		font-family: var(--font-display);
		font-size: 0.9rem;
		text-align: center;
		line-height: 1.05;
		overflow-wrap: anywhere;
	}
	.event-copy {
		display: grid;
		align-content: center;
		min-width: 0;
	}
	.event-copy h3 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1.08rem;
		overflow-wrap: anywhere;
	}
	.event-copy p {
		margin: 0.2rem 0 0;
		color: var(--ink-soft);
		font-size: 0.84rem;
		line-height: 1.4;
		overflow-wrap: anywhere;
	}
	.timeline li.positive h3 {
		color: var(--positive);
	}
	.timeline li.negative h3 {
		color: var(--critical);
	}
	@media (max-width: 560px) {
		.timeline {
			margin-top: 0.9rem;
		}
		.day-group {
			grid-template-columns: minmax(0, 1fr);
			gap: 0.7rem;
			padding: 0.85rem 0;
		}
		.day-marker {
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 0.75rem;
			min-height: 0;
			padding: 0.5rem 0.65rem;
		}
		.day-marker strong {
			font-size: 0.82rem;
			text-align: right;
			overflow-wrap: normal;
		}
		.day-events {
			gap: 0.65rem;
		}
		.event-copy h3 {
			font-size: 1rem;
			line-height: 1.15;
		}
		.event-copy p {
			font-size: 0.8rem;
		}
	}
	@media (max-width: 390px) {
		.day-marker {
			align-items: center;
		}
		.day-marker span {
			font-size: 0.58rem;
		}
		.day-marker strong {
			font-size: 0.78rem;
			line-height: 1.15;
		}
	}
</style>
