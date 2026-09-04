<script lang="ts">
	import type { ChronicleEntry } from '$lib/api/generated';
	import { describeChronicleEntry } from '$lib/domain/chronicle';

	let { entries }: { entries: ChronicleEntry[] } = $props();
	let grouped = $derived.by(() => {
		const groups: { tick: number; entries: ChronicleEntry[] }[] = [];
		for (const entry of entries) {
			const current = groups.at(-1);
			if (current?.tick === entry.occurred_tick) current.entries.push(entry);
			else groups.push({ tick: entry.occurred_tick, entries: [entry] });
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
			{#each grouped as group (group.tick)}
				<li class="tick-group">
					<div class="tick-marker"><span>Tick</span><strong>{group.tick}</strong></div>
					<ol class="tick-events">
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
	.tick-group {
		display: grid;
		grid-template-columns: 3.5rem 1fr;
		gap: 1rem;
		padding: 0.9rem 0;
		border-top: 1px solid var(--line-light);
	}
	.tick-group:first-child {
		border-top: 0;
	}
	.tick-events {
		display: grid;
		gap: 0.7rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.tick-events li {
		min-width: 0;
	}
	.tick-marker {
		display: grid;
		align-content: center;
		justify-items: center;
		min-height: 3.25rem;
		background: var(--surface-muted);
		color: var(--ink-soft);
	}
	.tick-marker span {
		font-size: 0.62rem;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.tick-marker strong {
		color: var(--ink);
		font-family: var(--font-display);
		font-size: 1.35rem;
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
