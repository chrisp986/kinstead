<script lang="ts">
	import type { PoliticalDecision, PoliticsOverview } from '$lib/api/generated';
	type Feedback = { action?: string; message?: string };
	let { politics, feedback } = $props<{
		politics: PoliticsOverview;
		householdId: string;
		feedback: Feedback | null;
	}>();
	const label = (d: PoliticalDecision) =>
		d.demand_type === 'political_labor_service' ? 'Labor service' : 'Levy';
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
			<article class="demand">
				<div>
					<h3>{demand.actor_name} · {label(demand)}</h3>
					<p>
						Respond before tick {demand.expires_tick}. {demand.status !== 'pending'
							? `Resolved: ${demand.selected_option}`
							: ''}
					</p>
				</div>
				{#if demand.status === 'pending'}
					<form method="POST" action="?/respondPoliticalDemand">
						<input type="hidden" name="decision_id" value={demand.id} />
						{#if demand.demand_type === 'political_labor_service'}
							<p>Serve for 4 ticks · +10 standing</p>
							<select name="character_id" required aria-label="Service character"
								><option value="">Choose a full-capacity member</option
								>{#each demand.eligible_characters ?? [] as character (character.id)}<option
										value={character.id}>{character.name}</option
									>{/each}</select
							>
							<button name="option" value="serve">Serve</button>
						{:else}
							<p>18 wood → +10 · 6 silver → +10 · Refuse → -5</p>
							<button name="option" value="pay_wood">Pay wood</button><button
								name="option"
								value="pay_silver">Pay silver</button
							>
						{/if}
						<button name="option" value="refuse" class="secondary">Refuse (-5)</button>
					</form>
				{/if}
			</article>
		{/each}
	{/if}
	{#if feedback?.action === 'respondPoliticalDemand'}<p class="feedback">{feedback.message}</p>{/if}
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
	.feedback {
		margin-top: 0.75rem;
		color: var(--accent);
	}
</style>
