<script lang="ts">
	import type { MarketOffer } from '$lib/api/generated';
	import { formatMilli, labelResource } from '$lib/domain/format';
	import { enhance } from '$app/forms';
	import ActionFeedback from './shell/ActionFeedback.svelte';
	import { calendarForGameDay, formatGameDay } from '$lib/domain/time';

	let {
		offers,
		householdId,
		feedback
	}: {
		offers: MarketOffer[];
		householdId: string;
		feedback?: {
			success?: boolean;
			action?: string;
			offerId?: string;
			message?: string;
			quote?: {
				goods_cost_milli: number;
				transport_cost_milli: number;
				total_cost_milli: number;
				remaining_silver_milli: number;
				expected_arrival_game_day: number;
				travel_ticks: number;
			};
		} | null;
	} = $props();
	let purchasing = $state<string | null>(null);
	let quantities = $state<Record<string, number>>({});
	let quoteTimers: Record<string, ReturnType<typeof setTimeout>> = {};
	function quantityFor(offer: MarketOffer): number {
		return quantities[offer.id] ?? Math.min(5, offer.quantity_remaining_milli / 1000);
	}
	function queueQuote(event: Event, offer: MarketOffer) {
		const form = event.currentTarget as HTMLFormElement;
		clearTimeout(quoteTimers[offer.id]);
		quoteTimers[offer.id] = setTimeout(() => form.requestSubmit(), 250);
	}
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
						<span class="offer-label">From {offer.seller_household_name || 'Nearby household'}</span
						>
						<h3>{labelResource(offer.resource_type)}</h3>
						<p>
							<strong>{formatMilli(offer.quantity_remaining_milli)}</strong> available ·
							<strong>{formatMilli(offer.price_per_unit_milli)}</strong> silver per unit
						</p>
					</div>
					<form
						method="POST"
						action="?/quote"
						oninput={(event) => queueQuote(event, offer)}
						use:enhance={() =>
							async ({ update }) =>
								update({ reset: false })}
					>
						<input type="hidden" name="offer_id" value={offer.id} />
						<label
							><span>Quantity</span><input
								type="number"
								name="quantity"
								min="0.001"
								max={formatMilli(offer.quantity_remaining_milli)}
								step="0.001"
								value={quantityFor(offer)}
								oninput={(event) => (quantities[offer.id] = Number(event.currentTarget.value))}
								required
							/></label
						>
					</form>
					{#if feedback?.action === 'quote' && feedback.offerId === offer.id}
						{#if feedback.quote}
							<div class="quote" aria-live="polite">
								<span
									>Goods <strong>{formatMilli(feedback.quote.goods_cost_milli)} silver</strong
									></span
								>
								<span
									>Transport <strong
										>{formatMilli(feedback.quote.transport_cost_milli)} silver</strong
									></span
								>
								<span
									>Total <strong>{formatMilli(feedback.quote.total_cost_milli)} silver</strong
									></span
								>
								<span
									>Silver after purchase <strong
										>{formatMilli(feedback.quote.remaining_silver_milli)}</strong
									></span
								>
								<span
									>Expected arrival <strong
										>{formatGameDay(
											calendarForGameDay(feedback.quote.expected_arrival_game_day)
										)}</strong
									></span
								>
							</div>
						{:else}<p class="quote-error" role="alert">{feedback.message}</p>{/if}
					{/if}
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
						<input type="hidden" name="quantity" value={quantityFor(offer)} />
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
	.quote {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.35rem 0.8rem;
		padding: 0.75rem;
		background: var(--surface-muted);
		font-size: 0.78rem;
	}
	.quote span {
		display: flex;
		justify-content: space-between;
		gap: 0.5rem;
	}
	.quote-error {
		color: var(--critical);
		font-size: 0.8rem;
		margin: 0;
	}
	@media (max-width: 480px) {
		.offer form {
			grid-template-columns: 1fr;
		}
		.quote {
			grid-template-columns: 1fr;
		}
	}
</style>
