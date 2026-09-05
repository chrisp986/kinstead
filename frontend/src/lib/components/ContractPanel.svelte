<script lang="ts">
	import { enhance } from '$app/forms';
	import type { Contract, MarketOffer, Relationship } from '$lib/api/generated';
	import { formatMilli, labelResource, shortId } from '$lib/domain/format';
	import { calendarForGameDay, formatInterval, formatRelativeGameDay } from '$lib/domain/time';
	import StatusBadge from './StatusBadge.svelte';
	import ActionFeedback from './shell/ActionFeedback.svelte';

	let {
		contracts,
		relationships,
		offers,
		householdId,
		currentGameDay,
		feedback
	}: {
		contracts: Contract[];
		relationships: Relationship[];
		offers: MarketOffer[];
		householdId: string;
		currentGameDay: number;
		feedback?: { success?: boolean; action?: string; message?: string } | null;
	} = $props();

	let submitting = $state<string | null>(null);
	let showProposal = $state(false);
	let orderedContracts = $derived(
		[...contracts].sort((a, b) => {
			const aUrgent = a.obligations.some(
				(o) => o.debtor_household_id === householdId && ['pending', 'late'].includes(o.status)
			);
			const bUrgent = b.obligations.some(
				(o) => o.debtor_household_id === householdId && ['pending', 'late'].includes(o.status)
			);
			if (aUrgent !== bUrgent) return aUrgent ? -1 : 1;
			if (a.status !== b.status) return a.status === 'proposed' ? -1 : 1;
			return a.id.localeCompare(b.id);
		})
	);
	let contacts = $derived.by(() => {
		const values: Record<string, string> = {};
		for (const relationship of relationships) {
			if (relationship.source_household_id === householdId) {
				values[relationship.target_household_id] = relationship.target_household_name;
			} else {
				values[relationship.source_household_id] = relationship.source_household_name;
			}
		}
		for (const offer of offers) {
			if (offer.seller_household_id !== householdId && !values[offer.seller_household_id]) {
				values[offer.seller_household_id] = `Household …${shortId(offer.seller_household_id)}`;
			}
		}
		return Object.entries(values).map(([id, name]) => ({ id, name }));
	});

	function counterpart(contract: Contract): string {
		return contract.party_a_household_id === householdId
			? contract.party_b_household_id
			: contract.party_a_household_id;
	}

	function agreementEnd(contract: Contract): string {
		const date = calendarForGameDay(contract.end_game_day);
		if (date.day_of_year === 91) return 'until summer begins';
		if (date.day_of_year === 273) return 'until winter begins';
		return `ends ${formatRelativeGameDay(currentGameDay, contract.end_game_day)}`;
	}
</script>

<section class="panel contract-panel" aria-labelledby="contracts-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Promises over time</p>
			<h2 id="contracts-heading">Contracts</h2>
		</div>
		<span class="count">{contracts.length}</span>
	</div>

	{#if feedback?.action && ['proposeContract', 'respondContract', 'dispatchObligation'].includes(feedback.action)}
		<ActionFeedback {feedback} />
	{/if}

	{#if contracts.length === 0}
		<p class="empty">No recurring promises involve this household yet.</p>
	{:else}
		<div class="contract-list">
			{#each orderedContracts as contract (contract.id)}
				<article class="contract">
					<div class="contract-heading">
						<div>
							<span class="contract-label">With household …{shortId(counterpart(contract))}</span>
							<h3>{formatInterval(contract.interval_days)} · recurring delivery</h3>
						</div>
						<StatusBadge status={contract.status} />
					</div>
					<ul class="terms">
						{#each contract.terms as term (`${term.debtor_household_id}-${term.creditor_household_id}-${term.resource_type}`)}
							<li>
								<strong
									>{formatMilli(term.quantity_milli)} {labelResource(term.resource_type)}</strong
								>
								<span
									>{term.debtor_household_id === householdId ? 'You deliver' : 'You receive'}</span
								>
							</li>
						{/each}
					</ul>
					<div class="contract-meta">
						<div>
							<span>Agreement</span><strong>{formatInterval(contract.interval_days)}</strong>
						</div>
						<div><span>Duration</span><strong>{agreementEnd(contract)}</strong></div>
					</div>

					{#if contract.status === 'proposed' && contract.party_b_household_id === householdId}
						<form method="POST" action="?/respondContract" class="response-actions" use:enhance>
							<input type="hidden" name="contract_id" value={contract.id} />
							<button name="decision" value="accept" type="submit">Accept promise</button>
							<button class="secondary" name="decision" value="reject" type="submit">Decline</button
							>
						</form>
					{/if}

					{#if contract.obligations.length > 0}
						<div class="obligations">
							{#each contract.obligations as obligation (obligation.id)}
								<div class="obligation">
									<div>
										<strong
											>Next delivery {formatRelativeGameDay(
												currentGameDay,
												obligation.due_game_day
											)}</strong
										>
										<span
											>{formatMilli(obligation.quantity_milli)}
											{labelResource(obligation.resource_type)}</span
										>
										{#if obligation.latest_dispatch_game_day !== undefined}
											<small
												>Latest safe dispatch
												{formatRelativeGameDay(
													currentGameDay,
													obligation.latest_dispatch_game_day
												)}</small
											>
										{/if}
									</div>
									<StatusBadge status={obligation.status} />
									{#if obligation.debtor_household_id === householdId && !obligation.shipment_id && ['pending', 'late'].includes(obligation.status)}
										<form
											method="POST"
											action="?/dispatchObligation"
											use:enhance={() => {
												submitting = obligation.id;
												return async ({ update }) => {
													await update();
													submitting = null;
												};
											}}
										>
											<input type="hidden" name="obligation_id" value={obligation.id} />
											<button type="submit" disabled={submitting !== null}>
												{submitting === obligation.id ? 'Dispatching…' : 'Dispatch goods'}
											</button>
										</form>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</article>
			{/each}
		</div>
	{/if}

	<div class="proposal">
		<button
			class="disclosure"
			type="button"
			aria-expanded={showProposal}
			onclick={() => (showProposal = !showProposal)}
		>
			{showProposal ? '− Hide proposal form' : '+ Propose recurring delivery'}
		</button>
		{#if showProposal}<h3>Offer a recurring delivery</h3>
			{#if contacts.length === 0}
				<p class="empty">
					A known market or relationship contact is needed before proposing a delivery.
				</p>
			{:else}
				<form method="POST" action="?/proposeContract" use:enhance>
					<label>
						<span>Counterparty</span>
						<select name="counterparty_household_id" required>
							{#each contacts as contact (contact.id)}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</label>
					<label>
						<span>You deliver</span>
						<select name="resource_type">
							<option value="provisions">Provisions</option>
							<option value="wood">Wood</option>
							<option value="trade_goods">Trade goods</option>
							<option value="silver">Silver</option>
						</select>
					</label>
					<label
						><span>Units each time</span><input
							name="quantity"
							type="number"
							min="0.001"
							step="0.001"
							value="10"
							required
						/></label
					>
					<input type="hidden" name="current_game_day" value={currentGameDay} />
					<label
						><span>First delivery</span><select name="first_due_offset" required
							><option value="7">In one week</option><option value="14">In two weeks</option><option
								value="28">In four weeks</option
							></select
						>
						/></label
					>
					<label
						><span>Repeat</span><select name="interval_days" required
							><option value="7">Every week</option><option value="14">Every two weeks</option
							><option value="28">Every four weeks</option></select
						>
						/></label
					>
					<label
						><span>Ends at</span><select name="end_condition_type" required
							><option value="fixed_delivery_count">After the delivery count</option><option
								value="winter_start">Winter begins</option
							><option value="summer_start">Summer begins</option></select
						>
						/></label
					>
					<label
						><span>Delivery count</span><select name="delivery_count" required
							><option value="4">After 4 deliveries</option><option value="8"
								>After 8 deliveries</option
							></select
						></label
					>
					<button type="submit">Send proposal</button>
				</form>
			{/if}{/if}
	</div>
</section>

<style>
	.contract-panel {
		grid-column: 1 / -1;
	}
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
	.contract-list {
		display: grid;
		gap: 0.9rem;
		margin-top: 1.2rem;
	}
	.contract {
		padding: 1rem;
		border: 1px solid var(--line-light);
	}
	.contract-heading {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: 1rem;
	}
	.contract-label {
		color: var(--ink-soft);
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.contract h3,
	.proposal h3 {
		margin: 0.2rem 0;
		font-family: var(--font-display);
		font-size: 1.15rem;
	}
	.terms {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem 1.5rem;
		margin: 0.8rem 0 0;
		padding: 0;
		list-style: none;
	}
	.terms li {
		display: grid;
		font-size: 0.83rem;
	}
	.terms span,
	.obligation span,
	.contract-meta span,
	.obligation small {
		color: var(--ink-soft);
	}
	.contract-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 1.25rem;
		margin-top: 0.85rem;
		padding-top: 0.7rem;
		border-top: 1px solid var(--line-light);
	}
	.contract-meta > div {
		display: grid;
		gap: 0.15rem;
		font-size: 0.78rem;
	}
	.contract-meta span {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.response-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: 1rem;
	}
	.secondary {
		border-color: var(--line);
		background: var(--surface-muted);
		color: var(--ink);
	}
	.obligations {
		display: grid;
		gap: 0.45rem;
		margin-top: 1rem;
	}
	.obligation {
		display: grid;
		grid-template-columns: minmax(9rem, 1fr) auto auto;
		align-items: center;
		gap: 0.7rem;
		padding-top: 0.55rem;
		border-top: 1px solid var(--line-light);
	}
	.obligation > div {
		display: grid;
		font-size: 0.8rem;
	}
	.obligation small {
		font-size: 0.72rem;
	}
	.obligation button {
		min-height: 2.1rem;
		padding: 0.35rem 0.7rem;
		font-size: 0.75rem;
	}
	.proposal {
		margin-top: 1.2rem;
		padding-top: 1.2rem;
		border-top: 1px solid var(--line);
	}
	.disclosure {
		width: 100%;
		margin: 0;
		background: transparent;
		border: 1px solid var(--line);
		color: var(--green);
	}
	.proposal form {
		display: grid;
		grid-template-columns: repeat(3, minmax(8rem, 1fr));
		align-items: end;
		gap: 0.7rem;
		margin-top: 0.8rem;
	}
	@media (max-width: 800px) {
		.proposal form {
			grid-template-columns: repeat(2, minmax(8rem, 1fr));
		}
	}
	@media (max-width: 560px) {
		.proposal form,
		.obligation {
			grid-template-columns: 1fr;
		}
		.response-actions {
			flex-direction: column;
		}
	}
</style>
