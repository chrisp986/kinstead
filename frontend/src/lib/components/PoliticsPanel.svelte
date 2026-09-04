<script lang="ts">
	import type { PoliticalDecision, PoliticsOverview } from '$lib/api/generated';
	import { formatMilli } from '$lib/domain/format';
	import { formatRelativeGameDay } from '$lib/domain/time';
	import ActionFeedback from './shell/ActionFeedback.svelte';
	type Feedback = { action?: string; message?: string };
	let { politics, currentGameDay, feedback } = $props<{
		politics: PoliticsOverview;
		currentGameDay: number;
		feedback: Feedback | null;
	}>();
	const label = (d: PoliticalDecision) =>
		d.demand_type === 'political_labor_service' ? 'Labor service' : 'Levy';
	const option = (d: PoliticalDecision, code: string) =>
		d.options.find((value) => value.code === code);
	const formatStandingDelta = (value: number) => `${value >= 0 ? '+' : ''}${value}`;
</script>

<section class="panel" aria-labelledby="politics-heading">
	<div class="panel-heading">
		<div>
			<p class="eyebrow">Politics</p>
			<h2 id="politics-heading">Jarl demands</h2>
		</div>
	</div>
	{#if politics.decisions.length === 0}<p class="muted">No current demands.</p>{:else}
		{#each politics.decisions as demand (demand.id)}
			{@const candidates = demand.eligible_characters ?? []}
			<article class="demand">
				<div>
					<h3>{demand.actor_name} · {label(demand)}</h3>
					<p>
						Respond {formatRelativeGameDay(currentGameDay, demand.expires_game_day)}. {demand.status !==
						'pending'
							? `Resolved: ${demand.selected_option}`
							: ''}
					</p>
				</div>
				{#if demand.status === 'pending'}
					{#if demand.demand_type === 'political_labor_service'}
						{@const serve = option(demand, 'serve')}
						<p>
							Serve for about four weeks · {formatStandingDelta(serve?.standing_delta ?? 0)} standing
						</p>
						<form method="POST" action="?/respondPoliticalDemand">
							<input type="hidden" name="decision_id" value={demand.id} />
							<select
								name="character_id"
								required
								disabled={candidates.length === 0}
								aria-label="Service character"
							>
								<option value="">Choose a full-capacity member</option>
								{#each candidates as character (character.id)}<option value={character.id}
										>{character.name}</option
									>{/each}
							</select>
							<button type="submit" name="option" value="serve" disabled={candidates.length === 0}
								>Serve</button
							>
						</form>
						{#if candidates.length === 0}<p class="muted">
								No household member is available for service.
							</p>{/if}
						<form method="POST" action="?/respondPoliticalDemand">
							<input type="hidden" name="decision_id" value={demand.id} />
							<input type="hidden" name="option" value="refuse" />
							<button type="submit" class="secondary">Refuse</button>
						</form>
					{:else}
						{@const wood = option(demand, 'pay_wood')}
						{@const silver = option(demand, 'pay_silver')}
						{@const refuse = option(demand, 'refuse')}
						<form method="POST" action="?/respondPoliticalDemand">
							<input type="hidden" name="decision_id" value={demand.id} />
							<p>
								{formatMilli(wood?.resource_milli ?? 0)}
								{wood?.resource_code ?? 'wood'} → {formatStandingDelta(wood?.standing_delta ?? 0)} standing
								· {formatMilli(silver?.resource_milli ?? 0)}
								{silver?.resource_code ?? 'silver'} → {formatStandingDelta(
									silver?.standing_delta ?? 0
								)} standing · Refuse → {formatStandingDelta(refuse?.standing_delta ?? 0)} standing
							</p>
							<button type="submit" name="option" value="pay_wood">Pay wood</button>
							<button type="submit" name="option" value="pay_silver">Pay silver</button>
							<button type="submit" name="option" value="refuse" class="secondary">
								Refuse ({formatStandingDelta(refuse?.standing_delta ?? 0)})
							</button>
						</form>
					{/if}
				{/if}
			</article>
		{/each}
	{/if}
	<ActionFeedback feedback={feedback?.action === 'respondPoliticalDemand' ? feedback : null} />
</section>

<style>
	.panel {
		padding: 1.25rem;
		background: var(--surface);
		border: 1px solid var(--line);
	}
	.panel-heading h2 {
		margin: 0.2rem 0 1rem;
		font-family: var(--font-display);
	}
	.demand {
		display: grid;
		gap: 0.75rem;
		padding: 0.9rem 0;
		border-top: 1px solid var(--line);
	}
	h3,
	p {
		margin: 0;
	}
	h3 {
		font-size: 1rem;
	}
	p {
		color: var(--ink-soft);
		font-size: 0.85rem;
	}
	form {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: center;
	}
	select {
		padding: 0.45rem;
	}
	button {
		border: 0;
		padding: 0.5rem 0.7rem;
		background: var(--ink);
		color: white;
		cursor: pointer;
	}
	button.secondary {
		background: transparent;
		color: var(--ink);
		border: 1px solid var(--line);
	}
</style>
