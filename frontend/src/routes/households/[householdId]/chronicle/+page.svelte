<script lang="ts">
	import ChronicleLog from '$lib/components/ChronicleLog.svelte';
	import PageHeader from '$lib/components/shell/PageHeader.svelte';

	let { data } = $props();
</script>

<svelte:head><title>{data.report.household_name} · Chronicle</title></svelte:head>

<main class="chronicle-page">
	<PageHeader
		eyebrow="Household memory"
		title="Chronicle"
		description="A clear record of the work, promises, arrivals, and decisions that shaped the farmstead."
	/>
	<ChronicleLog entries={data.chronicle} />
</main>
