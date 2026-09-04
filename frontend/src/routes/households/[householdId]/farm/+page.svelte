<script lang="ts">
	import HouseholdMembers from '$lib/components/farm/HouseholdMembers.svelte';
	import PoliticsPanel from '$lib/components/PoliticsPanel.svelte';
	import ResourceSummary from '$lib/components/ResourceSummary.svelte';
	import PageHeader from '$lib/components/shell/PageHeader.svelte';

	let { data, form } = $props();
</script>

<svelte:head><title>{data.report.household_name} · Farm</title></svelte:head>

<main class="farm-page">
	<PageHeader
		eyebrow="Farmstead"
		title="Farm"
		description="Stores, household members, and the obligations that shape this place."
	/>
	<ResourceSummary resources={data.report.resources} supplyDays={data.report.supply_days} />
	<HouseholdMembers
		characters={data.report.characters}
		assignments={data.report.assignments}
		currentGameDay={data.report.game_day}
	/>
	<div id="politics">
		<PoliticsPanel politics={data.politics} currentGameDay={data.report.game_day} feedback={form} />
	</div>
</main>
