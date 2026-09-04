<script lang="ts">
	import type { MarketOffer } from '$lib/api/generated';
	import { formatMilli, labelResource, shortId } from '$lib/domain/format';
	import { enhance } from '$app/forms';
	import ActionFeedback from './shell/ActionFeedback.svelte';

	let {
		offers,
		householdId,
		feedback
	}: {
		offers: MarketOffer[];
		householdId: string;
		feedback?: { success?: boolean; action?: string; message?: string } | null;
	} = $props();
	let purchasing = $state<string | null>(null);
</script>

<section class="panel" aria-labelledby="market-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Trade</p>
			<h2 id="market-heading">Nearby offers</h2>
		</div>
	</div>
	<ActionFeedback feedback={feedback?.action === 'purchase' ? feedback : null} />
	{#if offers.length === 0}
		<p class="empty">There are no active market offers.</p>
	{:else}
		<div class="offers">
			{#each offers as offer (offer.id)}
				<article class="offer">
					<div>
						<span class="offer-label">From household …{shortId(offer.seller_household_id)}</span>
						<h3>{labelResource(offer.resource_type)}</h3>
						<p>
							<strong>{formatMilli(offer.quantity_remaining_milli)}</strong> available ·
							<strong>{formatMilli(offer.price_per_unit_milli)}</strong> silver per unit
						</p>
					</div>
					<form
						method="POST"
						action="?/purchase"
						use:enhance={() => {
							purchasing = offer.id;
							return async ({ update }) => {
								await update();
								purchasing = null;
							};
						}}
					>
						<input type="hidden" name="offer_id" value={offer.id} />
						<label
							><span>Quantity</span><input
								type="number"
								name="quantity"
								min="0.001"
								max={formatMilli(offer.quantity_remaining_milli)}
								step="0.001"
								value={Math.min(5, offer.quantity_remaining_milli / 1000)}
								required
							/></label
						>
						<button
							type="submit"
							disabled={purchasing !== null || offer.seller_household_id === householdId}
							>{purchasing === offer.id ? 'Buying…' : 'Buy for delivery'}</button
						>
					</form>
				</article>
			{/each}
		</div>
	{/if}
</section>

<style>
	.offers {
		display: grid;
		gap: 0.8rem;
		margin-top: 1.25rem;
	}
	.offer {
		display: grid;
		gap: 1rem;
		padding: 1rem;
		border: 1px solid var(--line-light);
	}
	.offer-label {
		color: var(--ink-soft);
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.offer h3 {
		margin: 0.2rem 0;
		font-family: var(--font-display);
		font-size: 1.25rem;
	}
	.offer p {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.82rem;
	}
	.offer form {
		display: grid;
		grid-template-columns: minmax(7rem, 1fr) auto;
		align-items: end;
		gap: 0.6rem;
	}
	@media (max-width: 480px) {
		.offer form {
			grid-template-columns: 1fr;
		}
	}
</style>
