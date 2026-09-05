<script lang="ts">
	import ContractPanel from '$lib/components/ContractPanel.svelte';
	import MarketPanel from '$lib/components/MarketPanel.svelte';
	import RelationshipPanel from '$lib/components/RelationshipPanel.svelte';
	import ShipmentList from '$lib/components/ShipmentList.svelte';
	import PageHeader from '$lib/components/shell/PageHeader.svelte';

	let { data, form } = $props();
</script>

<svelte:head><title>{data.report.household_name} · Trade</title></svelte:head>

<main class="trade-page">
	<PageHeader
		eyebrow="Exchange and promises"
		title="Trade"
		description="Move from opportunity to agreement, journey, arrival, and the relationship consequence that follows."
	/>
	<nav class="trade-nav" aria-label="Trade sections">
		<a href="#market"><span>1</span> Market</a>
		<a href="#contracts"><span>2</span> Agreements</a>
		<a href="#transit"><span>3</span> Journeys</a>
		<a href="#relations"><span>4</span> People</a>
	</nav>
	<div id="market">
		<MarketPanel offers={data.offers} householdId={data.report.household_id} feedback={form} />
	</div>
	<div id="contracts">
		<ContractPanel
			contracts={data.contracts}
			relationships={data.relationships}
			offers={data.offers}
			householdId={data.report.household_id}
			currentGameDay={data.report.game_day}
			feedback={form}
		/>
	</div>
	<div id="transit">
		<ShipmentList
			shipments={data.shipments}
			householdId={data.report.household_id}
			currentGameDay={data.report.game_day}
		/>
	</div>
	<div id="relations">
		<RelationshipPanel relationships={data.relationships} householdId={data.report.household_id} />
	</div>
</main>

<style>
	.trade-page {
		display: grid;
		gap: 1rem;
		padding-bottom: 1rem;
	}
	.trade-nav {
		position: sticky;
		top: 0;
		z-index: 5;
		display: flex;
		gap: 0.25rem;
		overflow-x: auto;
		padding: 0.4rem;
		border: 1px solid var(--line-light);
		background: rgba(248, 244, 233, 0.96);
	}
	.trade-nav a {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: var(--tap-min);
		padding: 0.65rem 0.8rem;
		color: var(--ink-soft);
		font-size: 0.78rem;
		font-weight: 700;
		text-decoration: none;
		white-space: nowrap;
	}
	.trade-nav a span {
		display: grid;
		place-items: center;
		width: 1.25rem;
		height: 1.25rem;
		border: 1px solid var(--line);
		border-radius: 50%;
		font-size: 0.65rem;
	}
	.trade-nav a:hover,
	.trade-nav a:focus-visible {
		color: var(--green);
		background: var(--surface-muted);
	}
</style>
