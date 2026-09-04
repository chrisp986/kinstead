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
		return relationship.source_household_id === householdId ? 'Your trust' : 'Their trust in you';
	}

	function trustEffect(delta: number): string {
		return `${delta > 0 ? '+' : ''}${delta} trust`;
	}

	function tickValue(value: unknown): number | null {
		return typeof value === 'number' && Number.isSafeInteger(value) ? value : null;
	}

	function outcomeDetail(event: Relationship['events'][number]): string | null {
		const due = tickValue(event.data.due_game_day);
		const arrived = tickValue(
			event.data.actual_arrival_game_day ?? event.data.actual_fulfillment_game_day
		);
		if (due === null) return null;
		if (arrived !== null) return 'Delivered after the due date';
		if (event.event_type === 'contract_obligation_broken') return 'Due date passed · unresolved';
		return null;
	}
</script>

<section class="panel" aria-labelledby="relationships-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Reputation with others</p>
			<h2 id="relationships-heading">Relationships</h2>
		</div>
	</div>
	{#if relationships.length === 0}
		<p class="empty">Contract outcomes will build a readable relationship history.</p>
	{:else}
		<div class="relationship-list">
			{#each relationships as relationship (`${relationship.source_household_id}-${relationship.target_household_id}`)}
				<article>
					<div class="relationship-heading">
						<div>
							<span>{direction(relationship)}</span>
							<h3>
								{otherName(relationship)}
								<small>…{shortId(otherId(relationship))}</small>
							</h3>
						</div>
						<strong>{sentenceCase(relationship.standing)} · {relationship.trust}</strong>
					</div>
					{#if relationship.events.length > 0}
						<ul>
							{#each relationship.events.slice(0, 4) as event (event.id)}
								<li>
									<div class="event-copy">
										<span>Relationship history</span>
										<strong
											>{sentenceCase(event.event_type.replace('contract_obligation_', ''))}</strong
										>
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
		gap: 0.8rem;
		margin-top: 1.2rem;
	}
	.relationship-list article {
		padding: 0.9rem;
		border: 1px solid var(--line-light);
	}
	.relationship-heading {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: 1rem;
	}
	.relationship-heading span,
	small {
		color: var(--ink-soft);
		font-size: 0.7rem;
		font-weight: 400;
	}
	.relationship-heading h3 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: 1.15rem;
	}
	.relationship-heading > strong {
		color: var(--green);
		font-size: 0.78rem;
		white-space: nowrap;
	}
	ul {
		display: grid;
		gap: 0.25rem;
		margin: 0.7rem 0 0;
		padding: 0.7rem 0 0;
		border-top: 1px solid var(--line-light);
		list-style: none;
	}
	li {
		display: flex;
		justify-content: space-between;
		align-items: start;
		gap: 1rem;
		color: var(--ink-soft);
		font-size: 0.75rem;
	}
	.event-copy {
		display: grid;
		gap: 0.1rem;
	}
	.event-copy span {
		color: var(--ink);
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
</style>
