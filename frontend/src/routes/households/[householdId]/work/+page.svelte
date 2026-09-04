<script lang="ts">
	import WorkPlanner from '$lib/components/work/WorkPlanner.svelte';
	import PageHeader from '$lib/components/shell/PageHeader.svelte';

	let { data, form } = $props();
</script>

<svelte:head><title>{data.report.household_name} · Work planning</title></svelte:head>

<main class="work-page">
	<PageHeader
		eyebrow="Labor decisions"
		title="Work planning"
		description="See who is free, who is tired, and plan the next useful work."
	/>
	<WorkPlanner
		characters={data.report.characters}
		assignments={data.report.assignments}
		feedback={form}
	/>
</main>
