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

	function direction(shipment: Shipment): string {
		return shipment.receiver_household_id === householdId ? 'Incoming' : 'Outgoing';
	}
</script>

<section class="panel" aria-labelledby="shipments-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Goods in motion</p>
			<h2 id="shipments-heading">Journeys</h2>
		</div>
		<span class="count">{shipments.length}</span>
	</div>
	{#if shipments.length === 0}
		<p class="empty">No goods are currently recorded on the road for this household.</p>
	{:else}
		<div class="shipment-list">
			{#each shipments as shipment (shipment.id)}
				<article class="shipment">
					<div class="shipment-main">
						<div>
							<span class="direction">{direction(shipment)}</span>
							<strong
								>{formatMilli(shipment.quantity_milli)}
								{labelResource(shipment.resource_type)}</strong
							>
							<p>Shipment …{shortId(shipment.id)}</p>
						</div>
						<StatusBadge status={shipment.status} />
					</div>

					<div class="journey" aria-label="Shipment journey">
						<div class:complete={shipment.status !== 'prepared'} class="stage">
							<span class="marker" aria-hidden="true"></span>
							<small>Prepared</small>
						</div>
						<div
							class:complete={shipment.status === 'in_transit' || shipment.status === 'arrived'}
							class="stage"
						>
							<span class="marker" aria-hidden="true"></span>
							<small>On the road</small>
						</div>
						<div class:complete={shipment.status === 'arrived'} class="stage">
							<span class="marker" aria-hidden="true"></span>
							<small>Arrived</small>
						</div>
					</div>

					<div class="timing">
						<div>
							<span>Departure</span>
							<strong>{formatRelativeGameDay(currentGameDay, shipment.departure_game_day)}</strong>
						</div>
						<div>
							<span
								>{shipment.actual_arrival_game_day === undefined
									? 'Expected arrival'
									: 'Arrival'}</span
							>
							<strong>
								{shipment.actual_arrival_game_day === undefined
									? formatRelativeGameDay(currentGameDay, shipment.expected_arrival_game_day)
									: formatRelativeGameDay(currentGameDay, shipment.actual_arrival_game_day)}
							</strong>
						</div>
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
		min-width: 2rem;
		height: 2rem;
		padding: 0 0.45rem;
		border-radius: 1rem;
		background: var(--ink);
		color: var(--paper);
		font-weight: 700;
	}
	.shipment-list {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.8rem;
		margin-top: 1.25rem;
	}
	.shipment {
		padding: 1rem;
		border: 1px solid var(--line-light);
		border-left: 3px solid var(--ochre);
		background: var(--surface);
	}
	.shipment-main {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: 1rem;
	}
	.shipment-main > div:first-child {
		display: grid;
		gap: 0.15rem;
	}
	.direction {
		color: var(--ochre);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.shipment-main strong {
		font-family: var(--font-display);
		font-size: 1.08rem;
		font-weight: 600;
	}
	.shipment p {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.7rem;
	}
	.journey {
		position: relative;
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0;
		margin-top: 1rem;
	}
	.journey::before {
		position: absolute;
		top: 0.32rem;
		right: 16.5%;
		left: 16.5%;
		height: 1px;
		background: var(--line);
		content: '';
	}
	.stage {
		position: relative;
		z-index: 1;
		display: grid;
		justify-items: center;
		gap: 0.25rem;
		text-align: center;
	}
	.marker {
		width: 0.7rem;
		height: 0.7rem;
		border: 2px solid var(--line);
		border-radius: 50%;
		background: var(--surface);
	}
	.stage.complete .marker {
		border-color: var(--green);
		background: var(--green);
	}
	.stage small {
		color: var(--ink-soft);
		font-size: 0.67rem;
	}
	.stage.complete small {
		color: var(--ink);
		font-weight: 700;
	}
	.timing {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1px;
		margin-top: 0.9rem;
		border: 1px solid var(--line-light);
		background: var(--line-light);
	}
	.timing > div {
		display: grid;
		gap: 0.15rem;
		padding: 0.65rem;
		background: var(--surface-muted);
	}
	.timing span {
		color: var(--ink-soft);
		font-size: 0.66rem;
		font-weight: 800;
		text-transform: uppercase;
	}
	.timing strong {
		font-size: 0.76rem;
	}
	@media (max-width: 760px) {
		.shipment-list {
			grid-template-columns: 1fr;
		}
	}
</style>
