<script lang="ts">
	import { formatProjectedUnits, labelResource } from '$lib/domain/format';
	let { resources, supplyDays }: { resources: Record<string, number>; supplyDays: number } =
		$props();
	let supplyTone = $derived(supplyDays < 15 ? 'critical' : supplyDays <= 30 ? 'warning' : 'safe');
</script>

<section class="panel" aria-labelledby="resources-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Stores</p>
			<h2 id="resources-heading">Household resources</h2>
		</div>
		<div class="supply {supplyTone}">
			<strong>{formatProjectedUnits(supplyDays)}</strong><span>supply days</span>
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
	.resource-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.75rem;
		margin-top: 1.25rem;
	}
	.resource {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.9rem;
		border: 1px solid var(--line-light);
		background: var(--surface-muted);
	}
	.resource span {
		color: var(--ink-soft);
		font-size: 0.86rem;
	}
	.resource strong {
		font-family: var(--font-display);
		font-size: 1.45rem;
	}
	.supply {
		display: grid;
		min-width: 5.5rem;
		text-align: right;
	}
	.supply strong {
		font-family: var(--font-display);
		font-size: 1.65rem;
		line-height: 1;
	}
	.supply span {
		font-size: 0.72rem;
		text-transform: uppercase;
	}
	.supply.safe strong {
		color: var(--positive);
	}
	.supply.warning strong {
		color: var(--warning);
	}
	.supply.critical strong {
		color: var(--critical);
	}
</style>
