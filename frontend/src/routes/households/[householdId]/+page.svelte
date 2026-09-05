<script lang="ts">
	import FarmReportPanel from '$lib/components/FarmReportPanel.svelte';
	import PageHeader from '$lib/components/shell/PageHeader.svelte';

	let { data } = $props();
</script>

<svelte:head>
	<title>{data.report.household_name} · Household report</title>
	<meta name="description" content={`Decision briefing for ${data.report.household_name}.`} />
</svelte:head>

<main class="report-page">
	<PageHeader
		eyebrow="Return briefing"
		title="Household report"
		description="Start with the decisions that matter, then check risks, deadlines, and what changed while time passed."
	/>
	<FarmReportPanel report={data.report} />
</main>

<style>
	.report-page {
		display: grid;
		gap: 0.5rem;
		padding-bottom: 1rem;
	}
	:global(.report-page .report-panel) {
		border-top: 3px solid var(--green);
	}
</style>
