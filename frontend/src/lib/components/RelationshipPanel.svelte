<script lang="ts">
	import type { Relationship } from '$lib/api/generated';
	import { sentenceCase, shortId } from '$lib/domain/format';

	let { relationships, householdId }: { relationships: Relationship[]; householdId: string } =
		$props();

	function otherName(relationship: Relationship): string {
		return relationship.source_household_id === householdId
			? relationship.target_household_name
			: relationship.source_household_name;
	}

	function otherId(relationship: Relationship): string {
		return relationship.source_household_id === householdId
			? relationship.target_household_id
			: relationship.source_household_id;
	}

	function direction(relationship: Relationship): string {
		return relationship.source_household_id === householdId
			? 'Your trust in them'
			: 'Their trust in you';
	}

	function standingSummary(standing: string): string {
		if (standing === 'connected') return 'A strong tie built through repeated dealings.';
		if (standing === 'favorable') return 'Past dealings have built useful trust.';
		if (standing === 'disapproving') return 'The relationship is strained and needs care.';
		return 'The relationship is workable, but not yet secure.';
	}

	function trustEffect(delta: number): string {
		return `${delta > 0 ? '+' : ''}${delta} trust`;
	}

	function tickValue(value: unknown): number | null {
		return typeof value === 'number' && Number.isSafeInteger(value) ? value : null;
	}

	function eventLabel(eventType: string): string {
		if (eventType === 'contract_obligation_fulfilled') return 'Kept a delivery promise';
		if (eventType === 'contract_obligation_late') return 'Delivery arrived late';
		if (eventType === 'contract_obligation_broken') return 'Delivery promise was broken';
		return sentenceCase(eventType.replace('contract_obligation_', ''));
	}

	function outcomeDetail(event: Relationship['events'][number]): string | null {
		const due = tickValue(event.data.due_game_day);
		const arrived = tickValue(
			event.data.actual_arrival_game_day ?? event.data.actual_fulfillment_game_day
		);
		if (due === null) return null;
		if (arrived !== null && arrived <= due) return 'Arrived by the agreed deadline';
		if (arrived !== null) return 'Arrived after the agreed deadline';
		if (event.event_type === 'contract_obligation_broken')
			return 'Deadline passed without fulfillment';
		return null;
	}
</script>

<section class="panel" aria-labelledby="relationships-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">People and promises</p>
			<h2 id="relationships-heading">Relationships</h2>
		</div>
	</div>
	{#if relationships.length === 0}
		<p class="empty">Contract outcomes will build a readable relationship history.</p>
	{:else}
		<div class="relationship-list">
			{#each relationships as relationship (`${relationship.source_household_id}-${relationship.target_household_id}`)}
				<article>
					<header class="relationship-heading">
						<div>
							<span>{direction(relationship)}</span>
							<h3>{otherName(relationship)}</h3>
							<p>{standingSummary(relationship.standing)}</p>
						</div>
						<div class={`standing ${relationship.standing}`}>
							<strong>{sentenceCase(relationship.standing)}</strong>
							<small>Trust {relationship.trust}</small>
						</div>
					</header>
					<div class="identity-detail">Household …{shortId(otherId(relationship))}</div>
					{#if relationship.events.length > 0}
						<div class="history-heading">Recent history</div>
						<ul>
							{#each relationship.events.slice(0, 4) as event (event.id)}
								<li>
									<div class="event-copy">
										<strong>{eventLabel(event.event_type)}</strong>
										{#if outcomeDetail(event)}<small>{outcomeDetail(event)}</small>{/if}
									</div>
									<strong
										class:positive={event.trust_delta > 0}
										class:negative={event.trust_delta < 0}>{trustEffect(event.trust_delta)}</strong
									>
								</li>
							{/each}
						</ul>
					{/if}
				</article>
			{/each}
		</div>
	{/if}
</section>

<style>
	.relationship-list {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.8rem;
		margin-top: 1.2rem;
	}
	.relationship-list article {
		padding: 1rem;
		border: 1px solid var(--line-light);
		background: var(--surface);
	}
	.relationship-heading {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: 1rem;
	}
	.relationship-heading > div:first-child > span,
	.identity-detail {
		color: var(--ink-soft);
		font-size: 0.68rem;
		font-weight: 700;
		letter-spacing: 0.05em;
		text-transform: uppercase;
	}
	.relationship-heading h3 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: 1.25rem;
		font-weight: 500;
	}
	.relationship-heading p {
		max-width: 24rem;
		margin: 0.25rem 0 0;
		color: var(--ink-soft);
		font-size: 0.78rem;
		line-height: 1.35;
	}
	.standing {
		display: grid;
		justify-items: end;
		gap: 0.1rem;
		padding: 0.4rem 0.5rem;
		border: 1px solid var(--line-light);
		background: var(--surface-muted);
		white-space: nowrap;
	}
	.standing strong {
		color: var(--green);
		font-size: 0.75rem;
		text-transform: uppercase;
	}
	.standing.disapproving strong {
		color: var(--critical);
	}
	.standing small {
		color: var(--ink-soft);
		font-size: 0.66rem;
	}
	.identity-detail {
		margin-top: 0.7rem;
		padding-top: 0.6rem;
		border-top: 1px solid var(--line-light);
		font-weight: 400;
		letter-spacing: 0;
		text-transform: none;
	}
	.history-heading {
		margin-top: 0.75rem;
		color: var(--ink);
		font-family: var(--font-display);
		font-size: 0.9rem;
		font-weight: 600;
	}
	ul {
		display: grid;
		gap: 0.45rem;
		margin: 0.45rem 0 0;
		padding: 0;
		list-style: none;
	}
	li {
		display: flex;
		justify-content: space-between;
		align-items: start;
		gap: 1rem;
		padding-top: 0.45rem;
		border-top: 1px solid var(--line-light);
		font-size: 0.75rem;
	}
	.event-copy {
		display: grid;
		gap: 0.1rem;
	}
	.event-copy strong {
		color: var(--ink);
		font-weight: 700;
	}
	.event-copy small {
		color: var(--ink-soft);
		font-size: 0.7rem;
	}
	li > strong {
		white-space: nowrap;
	}
	li > strong.positive {
		color: var(--positive);
	}
	li > strong.negative {
		color: var(--critical);
	}
	@media (max-width: 760px) {
		.relationship-list {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 460px) {
		.relationship-heading {
			display: grid;
		}
		.standing {
			justify-items: start;
			width: fit-content;
		}
	}
</style>
