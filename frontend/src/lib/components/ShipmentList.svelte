<script lang="ts">
	import type { Shipment } from '$lib/api/generated';
	import { formatMilli, labelResource, shortId } from '$lib/domain/format';
	import { formatRelativeGameDay } from '$lib/domain/time';
	import StatusBadge from './StatusBadge.svelte';
	let {
		shipments,
		householdId,
		currentGameDay
	}: { shipments: Shipment[]; householdId: string; currentGameDay: number } = $props();
</script>

<section class="panel" aria-labelledby="shipments-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">On the road</p>
			<h2 id="shipments-heading">Shipments</h2>
		</div>
		<span class="count">{shipments.length}</span>
	</div>
	{#if shipments.length === 0}
		<p class="empty">No shipments have been recorded for this household.</p>
	{:else}
		<div class="shipment-list">
			{#each shipments as shipment (shipment.id)}
				<article class="shipment">
					<div class="shipment-main">
						<div>
							<strong
								>{formatMilli(shipment.quantity_milli)}
								{labelResource(shipment.resource_type)}</strong
							>
							<p>
								{shipment.receiver_household_id === householdId ? 'Incoming' : 'Outgoing'} · shipment
								…{shortId(shipment.id)}
							</p>
						</div>
						<StatusBadge status={shipment.status} />
					</div>
					<div class="journey">
						<span
							>Departed {formatRelativeGameDay(currentGameDay, shipment.departure_game_day)}</span
						><span aria-hidden="true">→</span>
						<span
							>{shipment.actual_arrival_game_day === undefined
								? `Expected ${formatRelativeGameDay(currentGameDay, shipment.expected_arrival_game_day)}`
								: `Arrived ${formatRelativeGameDay(currentGameDay, shipment.actual_arrival_game_day)}`}</span
						>
					</div>
				</article>
			{/each}
		</div>
	{/if}
</section>

<style>
	.count {
		display: grid;
		place-items: center;
		width: 2rem;
		height: 2rem;
		border-radius: 50%;
		background: var(--ink);
		color: var(--paper);
		font-weight: 700;
	}
	.shipment-list {
		display: grid;
		gap: 0.8rem;
		margin-top: 1.25rem;
	}
	.shipment {
		padding: 1rem;
		border-left: 3px solid var(--ochre);
		background: var(--surface-muted);
	}
	.shipment-main,
	.journey {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}
	.shipment p,
	.journey {
		margin: 0.25rem 0 0;
		color: var(--ink-soft);
		font-size: 0.8rem;
	}
	.journey {
		justify-content: flex-start;
		margin-top: 0.8rem;
		padding-top: 0.7rem;
		border-top: 1px solid var(--line-light);
	}
</style>
