<script lang="ts">
	import { formatProjectedUnits, labelResource } from '$lib/domain/format';
	let { resources, supplyDays }: { resources: Record<string, number>; supplyDays: number } =
		$props();
	let supplyTone = $derived(
		supplyDays < 7
			? 'emergency'
			: supplyDays < 15
				? 'critical'
				: supplyDays <= 30
					? 'warning'
					: 'safe'
	);
	let supplyState = $derived(
		supplyDays < 7
			? 'Emergency'
			: supplyDays < 15
				? 'Critical'
				: supplyDays <= 30
					? 'Strained'
					: 'Secure'
	);
	let supplyMessage = $derived(
		supplyDays < 7
			? 'Food security needs immediate action.'
			: supplyDays < 15
				? 'Plan food work or trade before reserves become an emergency.'
				: supplyDays <= 30
					? 'Reserves are adequate, but the household has little margin.'
					: 'The household has a comfortable provisions buffer.'
	);
</script>

<section class="panel" aria-labelledby="resources-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Stores</p>
			<h2 id="resources-heading">Household resources</h2>
		</div>
	</div>

	<div class={`supply-callout ${supplyTone}`}>
		<div>
			<span class="supply-label">Provisions last</span>
			<strong>{formatProjectedUnits(supplyDays)} days</strong>
		</div>
		<div class="supply-copy">
			<span class="state">{supplyState}</span>
			<p>{supplyMessage}</p>
		</div>
	</div>

	<div class="resource-grid">
		{#each Object.entries(resources) as [resource, quantity] (resource)}
			<div class="resource">
				<span>{labelResource(resource)}</span><strong>{formatProjectedUnits(quantity)}</strong>
			</div>
		{/each}
	</div>
</section>

<style>
	.supply-callout {
		display: grid;
		grid-template-columns: minmax(8rem, 0.75fr) minmax(0, 1.25fr);
		gap: 1rem;
		align-items: center;
		margin-top: 1rem;
		padding: 1rem;
		border: 1px solid var(--line-light);
		border-left: 4px solid var(--positive);
		background: var(--surface-muted);
	}
	.supply-callout.warning {
		border-left-color: var(--warning);
	}
	.supply-callout.critical,
	.supply-callout.emergency {
		border-left-color: var(--critical);
	}
	.supply-callout.emergency {
		background: color-mix(in srgb, var(--critical) 7%, var(--surface));
	}
	.supply-label {
		display: block;
		color: var(--ink-soft);
		font-size: 0.7rem;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.supply-callout strong {
		display: block;
		margin-top: 0.1rem;
		color: var(--positive);
		font-family: var(--font-display);
		font-size: clamp(1.7rem, 5vw, 2.2rem);
		font-weight: 500;
		line-height: 1;
	}
	.supply-callout.warning strong {
		color: var(--warning);
	}
	.supply-callout.critical strong,
	.supply-callout.emergency strong {
		color: var(--critical);
	}
	.supply-copy {
		display: grid;
		gap: 0.2rem;
	}
	.state {
		color: var(--ink);
		font-size: 0.78rem;
		font-weight: 800;
		text-transform: uppercase;
	}
	.supply-copy p {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.82rem;
		line-height: 1.4;
	}
	.resource-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.75rem;
		margin-top: 0.9rem;
	}
	.resource {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.9rem;
		border: 1px solid var(--line-light);
		background: var(--surface);
	}
	.resource span {
		color: var(--ink-soft);
		font-size: 0.86rem;
	}
	.resource strong {
		font-family: var(--font-display);
		font-size: 1.45rem;
	}
	@media (max-width: 560px) {
		.supply-callout {
			grid-template-columns: 1fr;
			gap: 0.55rem;
		}
		.resource-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
