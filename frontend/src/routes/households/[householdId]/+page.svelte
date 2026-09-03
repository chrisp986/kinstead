<script lang="ts">
	import AssignmentPanel from '$lib/components/AssignmentPanel.svelte';
	import ChronicleLog from '$lib/components/ChronicleLog.svelte';
	import ContractPanel from '$lib/components/ContractPanel.svelte';
	import MarketPanel from '$lib/components/MarketPanel.svelte';
	import ResourceSummary from '$lib/components/ResourceSummary.svelte';
	import RelationshipPanel from '$lib/components/RelationshipPanel.svelte';
	import ShipmentList from '$lib/components/ShipmentList.svelte';
	import { resolve } from '$app/paths';
	let { data, form } = $props();
</script>

<svelte:head>
	<title>{data.report.household_name} · Household overview</title>
	<meta
		name="description"
		content={`Manage work, supplies, and deliveries for ${data.report.household_name}.`}
	/>
</svelte:head>

<header class="hero">
	<div>
		<p class="eyebrow">Household seat</p>
		<h1>{data.report.household_name}</h1>
		<p class="date">{data.report.historical_date} · {data.report.season}</p>
	</div>
	<div class="tick"><span>World tick</span><strong>{data.report.tick}</strong></div>
</header>

{#if data.report.alerts.length > 0}
	<section class="alerts" aria-labelledby="alerts-heading">
		<h2 id="alerts-heading">Needs attention</h2>
		<ul>
			{#each data.report.alerts as alert (alert.code)}<li
					class:critical={alert.level === 'critical'}
				>
					<span aria-hidden="true">{alert.level === 'critical' ? '!' : '•'}</span>{alert.message}
				</li>{/each}
		</ul>
	</section>
{/if}

<main class="dashboard">
	<ResourceSummary resources={data.report.resources} supplyDays={data.report.supply_days} />
	<MarketPanel offers={data.offers} householdId={data.report.household_id} feedback={form} />
	<ContractPanel
		contracts={data.contracts}
		relationships={data.relationships}
		offers={data.offers}
		householdId={data.report.household_id}
		currentTick={data.report.tick}
		feedback={form}
	/>
	<AssignmentPanel
		characters={data.report.characters}
		assignments={data.report.assignments}
		feedback={form}
	/>
	<ShipmentList shipments={data.shipments} householdId={data.report.household_id} />
	<RelationshipPanel relationships={data.relationships} householdId={data.report.household_id} />
	<ChronicleLog entries={data.chronicle} />
</main>

<footer class="page-footer">
	<p>
		State updates when commands complete or the page is refreshed. The world advances on the backend
		clock.
	</p>
	<a
		href={resolve('/households/[householdId]', { householdId: data.report.household_id })}
		data-sveltekit-reload>Refresh household</a
	>
</footer>

<style>
	.hero {
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: 2rem;
		padding: 3.75rem 0 2rem;
		border-bottom: 1px solid var(--line);
	}
	.hero h1 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: clamp(3rem, 8vw, 5.7rem);
		font-weight: 500;
		letter-spacing: -0.04em;
		line-height: 0.92;
	}
	.date {
		margin: 0.75rem 0 0;
		color: var(--ink-soft);
		text-transform: capitalize;
	}
	.tick {
		display: grid;
		min-width: 6rem;
		text-align: right;
	}
	.tick span {
		color: var(--ink-soft);
		font-size: 0.72rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.tick strong {
		font-family: var(--font-display);
		font-size: 3rem;
		line-height: 1;
	}
	.alerts {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 1.5rem;
		margin: 1.5rem 0 0;
		padding: 1rem 1.15rem;
		background: #f3e7cf;
		border-left: 4px solid var(--warning);
	}
	.alerts h2 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1rem;
	}
	.alerts ul {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem 1.4rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.alerts li {
		color: #66502e;
		font-size: 0.85rem;
	}
	.alerts li span {
		display: inline-grid;
		place-items: center;
		width: 1.25rem;
		height: 1.25rem;
		margin-right: 0.35rem;
		border-radius: 50%;
		background: var(--warning);
		color: white;
		font-weight: 800;
	}
	.alerts li.critical {
		color: var(--critical);
	}
	.alerts li.critical span {
		background: var(--critical);
	}
	.dashboard {
		display: grid;
		grid-template-columns: minmax(0, 1.08fr) minmax(18rem, 0.92fr);
		gap: 1rem;
		padding: 1rem 0;
	}
	.page-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.25rem 0 3rem;
		border-top: 1px solid var(--line);
		color: var(--ink-soft);
		font-size: 0.8rem;
	}
	.page-footer p {
		margin: 0;
	}
	.page-footer a {
		color: var(--green);
		font-weight: 700;
	}
	@media (max-width: 900px) {
		.dashboard {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 600px) {
		.hero {
			align-items: start;
			padding-top: 2.5rem;
		}
		.alerts {
			grid-template-columns: 1fr;
			gap: 0.5rem;
		}
		.page-footer {
			align-items: flex-start;
			flex-direction: column;
		}
	}
</style>
